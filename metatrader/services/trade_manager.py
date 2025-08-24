import json
import time
import asyncio
import hashlib
import MetaTrader5 as mt5
from websockets.exceptions import ConnectionClosed
from models.trade import PoolTrade
from services.mt5_client import MT5Client
from factories.trade_factory import TradeFactory
from repositories.trade_repository import TradeRepository
from strategies.trade_strategy import MarketTradeStrategy, PendingTradeStrategy
from config.settings import settings
from utils.logger import logger
import redis


class TradeManager:
    def __init__(self, mt5_client: MT5Client, trade_repository: TradeRepository, trade_factory: TradeFactory):
        self.mt5_client = mt5_client
        self.trade_repository = trade_repository
        self.trade_factory = trade_factory
        self.strategies = {
            "MARKET": MarketTradeStrategy(),
            "BUY_LIMIT": PendingTradeStrategy(),
            "SELL_LIMIT": PendingTradeStrategy(),
            "BUY_STOP": PendingTradeStrategy(),
            "SELL_STOP": PendingTradeStrategy()
        }
        self.redis_client = redis.Redis(
            host=settings.REDIS_HOST, port=settings.REDIS_PORT, db=0)
        self.load_trades_from_redis()

    def load_trades_from_redis(self):
        try:
            trade_keys = self.redis_client.keys("trade:*:*:*")  # Updated to match exact key format
            for key in trade_keys:
                trade_data = self.redis_client.get(key)
                if trade_data:
                    trade_dict = json.loads(trade_data.decode())
                    trade = self.trade_factory.create_trade(trade_dict)
                    if self.validate_trade(trade):
                        self.trade_repository.add_to_pool(trade)
                        logger.info(f"Loaded trade {trade.trade_id} from Redis")
                    else:
                        logger.warning(f"Invalid trade loaded from Redis: {trade.trade_id}")
                        self.redis_client.delete(key)
        except Exception as e:
            logger.error(f"Error loading trades from Redis: {str(e)}")

    def save_trade_to_redis(self, trade: PoolTrade):
        try:
            trade_key = f"trade:{trade.trade_id}:{trade.user_id}:{trade.account_type}"
            trade_data = trade.model_dump_json()
            self.redis_client.set(trade_key, trade_data)
            logger.info(f"Saved trade {trade.trade_id} to Redis")
        except Exception as e:
            logger.error(f"Error saving trade {trade.trade_id} to Redis: {str(e)}")

    def remove_trade_from_redis(self, trade: PoolTrade):
        try:
            trade_key = f"trade:{trade.trade_id}:{trade.user_id}:{trade.account_type}"
            self.redis_client.delete(trade_key)
            logger.info(f"Removed trade {trade.trade_id} from Redis")
        except Exception as e:
            logger.error(f"Error removing trade {trade.trade_id} from Redis: {str(e)}")

    def get_timestamp(self):
        tick = self.mt5_client.get_symbol_tick(settings.SYMBOL)
        return tick.time if tick else time.time()

    def validate_trade(self, trade: PoolTrade) -> bool:
        if trade.trade_type.upper() not in ["BUY", "SELL"]:
            return False
        if trade.order_type.upper() not in ["MARKET", "BUY_LIMIT", "SELL_LIMIT", "BUY_STOP", "SELL_STOP"]:
            return False
        if trade.account_type.lower() not in ["demo", "real"]:
            return False
        if trade.leverage <= 0 or trade.volume <= 0:
            return False
        if trade.order_type.upper() != "MARKET" and trade.entry_price < 0:
            return False
        if trade.order_type.upper() == "MARKET" and trade.entry_price > 0:
            return False
        if trade.stop_loss < 0 or trade.take_profit < 0:
            return False
        current_time = int(self.get_timestamp())
        if trade.expiration > 0 and trade.expiration <= current_time:
            return False
        return True

    async def handle_trade_request(self, json_data: dict, ws) -> bool:
        symbol = json_data.get("symbol", "")
        symbol_info = self.mt5_client.get_symbol_info(symbol)
        if not symbol_info:
            await self.send_trade_response(json_data.get("trade_id", ""), json_data.get("trade_code", ""), json_data.get("user_id", ""),
                                          "FAILED", "", ws, error="Invalid symbol")
            return False

        volume = json_data.get("volume", 0.0)
        if volume < symbol_info.volume_min or volume > symbol_info.volume_max:
            await self.send_trade_response(json_data.get("trade_id", ""), json_data.get("trade_code"), json_data.get("user_id", ""),
                                          "FAILED", "", ws, error="Invalid volume")
            return False

        trade = self.trade_factory.create_trade(json_data)
        if not self.validate_trade(trade):
            await self.send_trade_response(trade.trade_id, trade.trade_code, trade.user_id, "FAILED", "", ws, error="Invalid trade parameters")
            return False

        user_balance = await self.trade_repository.get_user_balance(trade.user_id, trade.account_type, trade.account_name, ws)
        required_margin = (volume * symbol_info.trade_contract_size *
                          self.mt5_client.get_symbol_tick(symbol).bid) / trade.leverage
        if user_balance < required_margin:
            await self.send_trade_response(trade.trade_id, trade.trade_code, trade.user_id, "FAILED", "", ws, error="Insufficient user balance")
            return False

        if not self.mt5_client.check_margin(trade.symbol, volume, trade.trade_type):
            await self.send_trade_response(trade.trade_id, trade.trade_code, trade.user_id, "FAILED", "", ws, error="Insufficient account margin")
            return False

        if trade.order_type == "MARKET":
            strategy = self.strategies.get("MARKET")
            if strategy is None:
                await self.send_trade_response(trade.trade_id, trade.trade_code, trade.user_id, "FAILED", "", ws, error="No strategy for MARKET")
                return False
            
            success, status = strategy.execute(trade, self.mt5_client)
            if success:
                await self.send_trade_response(trade.trade_id, status, trade.user_id, "EXECUTED", "", ws)
                self.save_trade_to_redis(trade)
                return True
            else:
                logger.warning(f"Market order {trade.trade_id} failed direct execution")
                await self.send_trade_response(trade.trade_id, status, trade.user_id, "FAILED", "", ws, error="Market execution failed")
                return False

        match_index = self.trade_repository.find_matching_trade(trade)
        if match_index >= 0:
            await self.execute_matched_trades(trade, self.trade_repository.pool[match_index], ws)
            if trade.trade_id != "":
                self.trade_repository.add_to_pool(trade)
                await self.send_trade_response(trade.trade_id, trade.trade_code, trade.user_id, "PENDING", "", ws)
        else:
            self.trade_repository.add_to_pool(trade)
            await self.send_trade_response(trade.trade_id, trade.trade_code, trade.user_id, "PENDING", "", ws)
            if trade.order_type in ["BUY_LIMIT", "SELL_LIMIT", "BUY_STOP", "SELL_STOP"]:
                strategy = self.strategies.get(trade.order_type)
                success, status = strategy.execute(trade, self.mt5_client)
                if strategy and success and trade.trade_id != "":
                    await self.send_trade_response(trade.trade_id, status, trade.user_id, "EXECUTED", "", ws)
                    self.save_trade_to_redis(trade)
                else:
                    logger.warning(f"Pending order {trade.trade_id} failed execution, remains PENDING")
        return True

    async def execute_matched_trades(self, trade1: PoolTrade, trade2: PoolTrade, ws):
        match_volume = min(trade1.volume, trade2.volume)

        symbol_info = self.mt5_client.get_symbol_info(trade1.symbol)
        if not symbol_info:
            await self.send_trade_response(trade1.trade_id, trade1.trade_code, trade1.user_id, "FAILED", trade2.trade_id, ws, error="Failed to get symbol info")
            await self.send_trade_response(trade2.trade_id, trade2.trade_code, trade2.user_id, "FAILED", trade1.trade_id, ws, error="Failed to get symbol info")
            return

        tick_size = getattr(symbol_info, 'trade_tick_size', 0.1)
        try:
            stops_level = symbol_info.stops_level
        except AttributeError:
            stops_level = 1000
        min_sl_distance = stops_level * tick_size

        tick = self.mt5_client.get_symbol_tick(trade1.symbol)

        def round_price(price: float) -> float:
            return round(price / tick_size) * tick_size

        if not self.mt5_client.check_margin(trade1.symbol, match_volume, trade1.trade_type):
            await self.send_trade_response(trade1.trade_id, trade1.trade_code, trade1.user_id, "FAILED", trade2.trade_id, ws, error="Insufficient margin for trade1")
            return
        if not self.mt5_client.check_margin(trade2.symbol, match_volume, trade2.trade_type):
            await self.send_trade_response(trade2.trade_id, trade2.trade_code, trade2.user_id, "FAILED", trade1.trade_id, ws, error="Insufficient margin for trade2")
            return

        if trade1.order_type == "MARKET" or trade2.order_type == "MARKET":
            if not tick:
                await self.send_trade_response(trade1.trade_id, trade1.trade_code, trade1.user_id, "FAILED", trade2.trade_id, ws, error="Failed to get market price")
                await self.send_trade_response(trade2.trade_id, trade2.trade_code, trade2.user_id, "FAILED", trade1.trade_id, ws, error="Failed to get market price")
                return
            match_price = round_price(tick.bid if trade1.trade_type == "SELL" else tick.ask)
        else:
            if trade1.order_type == "BUY_LIMIT" and trade2.order_type == "SELL_LIMIT":
                match_price = round_price(trade2.entry_price)
            elif trade1.order_type == "SELL_LIMIT" and trade2.order_type == "BUY_LIMIT":
                match_price = round_price(trade2.entry_price)
            else:
                match_price = round_price(min(trade1.entry_price, trade2.entry_price))

        def validate_sl_tp(trade: PoolTrade, price: float) -> tuple[float, float]:
            sl = round_price(trade.stop_loss) if trade.stop_loss else 0.0
            tp = round_price(trade.take_profit) if trade.take_profit else 0.0
            if stops_level == 0:
                return sl, tp
            if sl:
                sl_distance = abs(price - sl)
                if sl_distance < min_sl_distance:
                    sl = price + min_sl_distance if trade.trade_type == "SELL" else price - min_sl_distance
                    sl = round_price(sl)
            if tp:
                tp_distance = abs(price - tp)
                if tp_distance < min_sl_distance:
                    tp = price - min_sl_distance if trade.trade_type == "SELL" else price + min_sl_distance
                    tp = round_price(tp)
            return sl, tp

        sl1, tp1 = validate_sl_tp(trade1, match_price)
        sl2, tp2 = validate_sl_tp(trade2, match_price)

        try:
            filling_mode1 = self.mt5_client.get_symbol_filling_mode(trade1.symbol)
            filling_mode2 = self.mt5_client.get_symbol_filling_mode(trade2.symbol)
        except ValueError as e:
            await self.send_trade_response(trade1.trade_id, trade1.trade_code, trade1.user_id, "FAILED", trade2.trade_id, ws, error=str(e))
            await self.send_trade_response(trade2.trade_id, trade1.trade_code, trade2.user_id, "FAILED", trade1.trade_id, ws, error=str(e))
            return

        def generate_magic(trade_id: str) -> int:
            hash_object = hashlib.md5(trade_id.encode())
            hash_int = int(hash_object.hexdigest(), 16)
            return int(hash_int % 0xFFFFFFFF)

        magic1 = int(generate_magic(trade1.trade_id))
        magic2 = int(generate_magic(trade2.trade_id))

        pending_order_types = ["BUY_LIMIT", "SELL_LIMIT", "BUY_STOP", "SELL_STOP"]
        for trade in [trade1, trade2]:
            if trade.order_type in pending_order_types and trade.ticket != 0:
                success = self.mt5_client.close_order(
                    trade.ticket, trade.symbol, trade.volume,
                    {
                        "BUY_LIMIT": mt5.ORDER_TYPE_BUY_LIMIT,
                        "SELL_LIMIT": mt5.ORDER_TYPE_SELL_LIMIT,
                        "BUY_STOP": mt5.ORDER_TYPE_BUY_STOP,
                        "SELL_STOP": mt5.ORDER_TYPE_SELL_STOP
                    }[trade.order_type]
                )
                if success:
                    trade.ticket = 0
                else:
                    await self.send_trade_response(trade.trade_id, trade.trade_code, trade.user_id, "FAILED", "", ws, error="Failed to close pending order")
                    return

        request1 = {
            "action": mt5.TRADE_ACTION_DEAL,
            "symbol": trade1.symbol,
            "volume": match_volume,
            "type": mt5.ORDER_TYPE_BUY if trade1.trade_type == "BUY" else mt5.ORDER_TYPE_SELL,
            "price": match_price,
            "sl": sl1,
            "tp": tp1,
            "comment": "TradeMatch",
            "type_time": mt5.ORDER_TIME_GTC,
            "type_filling": filling_mode1,
            "magic": int(magic1)
        }
        request2 = {
            "action": mt5.TRADE_ACTION_DEAL,
            "symbol": trade2.symbol,
            "volume": match_volume,
            "type": mt5.ORDER_TYPE_BUY if trade2.trade_type == "BUY" else mt5.ORDER_TYPE_SELL,
            "price": match_price,
            "sl": sl2,
            "tp": tp2,
            "comment": "TradeMatch",
            "type_time": mt5.ORDER_TIME_GTC,
            "type_filling": filling_mode2,
            "magic": int(magic2)
        }

        success = True
        ticket1, ticket2 = 0, 0
        error = ""

        message1, result1 = self.mt5_client.order_send(request1)
        if result1 and result1.retcode == 10009:
            ticket1 = result1.order
            trade1.ticket = ticket1
            trade1.magic = int(magic1)
        else:
            success = False
            error = message1

        message2, result2 = self.mt5_client.order_send(request2)
        if result2 and result2.retcode == 10009:
            ticket2 = result2.order
            trade2.ticket = ticket2
            trade2.magic = int(magic2)
        else:
            success = False
            error = message2 if not error else error

        status = "MATCHED" if success else "FAILED"
        await self.send_trade_response(
            trade1.trade_id, trade1.trade_code, trade1.user_id, status, trade2.trade_id, ws,
            error=error, matched_volume=match_volume, remaining_volume=trade1.volume - match_volume
        )
        await self.send_trade_response(
            trade2.trade_id, trade2.trade_code, trade2.user_id, status, trade1.trade_id, ws,
            error=error, matched_volume=match_volume, remaining_volume=trade2.volume - match_volume
        )

        if success:
            trade1.volume -= match_volume
            trade2.volume -= match_volume

            if trade2.volume <= 0:
                trade2.status = "EXECUTED"
                self.trade_repository.remove_from_pool(trade2)
                self.remove_trade_from_redis(trade2)
            else:
                self.save_trade_to_redis(trade2)

            if trade1.volume <= 0:
                trade1.status = "EXECUTED"
                self.trade_repository.remove_from_pool(trade1)
                self.remove_trade_from_redis(trade1)
            else:
                self.save_trade_to_redis(trade1)

                remaining_request = {
                    "action": mt5.TRADE_ACTION_DEAL,
                    "symbol": trade1.symbol,
                    "volume": trade1.volume,
                    "type": mt5.ORDER_TYPE_BUY if trade1.trade_type == "BUY" else mt5.ORDER_TYPE_SELL,
                    "price": match_price,
                    "sl": sl1,
                    "tp": tp1,
                    "comment": "TradeMatch_Remaining",
                    "type_time": mt5.ORDER_TIME_GTC,
                    "type_filling": filling_mode1,
                    "magic": int(generate_magic(trade1.trade_id + "_remaining"))
                }
                message_rem, result_rem = self.mt5_client.order_send(remaining_request)
                if result_rem and result_rem.retcode == 10009:
                    trade1.ticket = result_rem.order
                    trade1.magic = remaining_request["magic"]
                    trade1.status = "EXECUTED"
                    self.trade_repository.remove_from_pool(trade1)
                    self.remove_trade_from_redis(trade1)
                else:
                    await self.send_trade_response(
                        trade1.trade_id, trade1.trade_code, trade1.user_id, "FAILED", None, ws,
                        error=message_rem, matched_volume=0, remaining_volume=trade1.volume
                    )

    async def send_trade_response(self, trade_id: str, trade_code: int, user_id: str, status: str, matched_trade_id: str, ws, error: str = None, matched_volume: float = 0, remaining_volume: float = 0):
        response = self.trade_factory.create_trade_response(
            trade_id, trade_code, user_id, status, matched_volume, matched_trade_id, remaining_volume)
        if error:
            response.status = error
        try:
            await ws.send(json.dumps(response.model_dump()))
        except ConnectionClosed:
            self.trade_repository.queue_message(
                json.dumps(response.model_dump()))
        except Exception as e:
            self.trade_repository.queue_message(
                json.dumps(response.model_dump()))

    async def handle_balance_request(self, json_data: dict, ws):
        user_id = json_data.get("user_id", "")
        account_type = json_data.get("account_type", "")
        balance = await self.trade_repository.get_user_balance(user_id, account_type, trade.account_name, ws)
        response = self.trade_factory.create_balance_response(
            user_id, account_type, balance)
        try:
            await ws.send(json.dumps(response.model_dump()))
        except ConnectionClosed:
            self.trade_repository.queue_message(
                json.dumps(response.model_dump()))
        except Exception as e:
            self.trade_repository.queue_message(
                json.dumps(response.model_dump()))

    async def handle_close_trade_request(self, json_data: dict, ws):
        user_id = json_data.get("user_id", "")
        trade_id = json_data.get("trade_id", "")
        account_type = json_data.get("account_type", "")
        success = False
        close_price = 0.0
        close_reason = ""
        profit = 0.0
        positions = self.mt5_client.get_positions()
        for position in positions:
            if position.comment == "Trade" and self.trade_repository.is_user_trade(user_id, trade_id, account_type):
                success = self.mt5_client.close_order(
                    position.ticket, position.symbol, position.volume, position.type)
                if success:
                    deals = self.mt5_client.history_deals_get(
                        position=position.ticket)
                    for deal in deals:
                        if deal.entry == mt5.DEAL_ENTRY_OUT:
                            profit = deal.profit
                            break
                    self.remove_trade_from_redis(PoolTrade(trade_id=trade_id, user_id=user_id, account_type=account_type))
                break
        else:
            orders = self.mt5_client.get_orders()
            success = False
            close_reason = ""
            for order in orders:
                if order.comment == "Pending" and self.trade_repository.is_user_trade(user_id, trade_id, account_type):
                    success = self.mt5_client.close_order(
                        order.ticket, order.symbol, order.volume_current, order.type)
                    close_reason = "CANCELED" if success else "FAILED 16"
                    for trade in self.trade_repository.pool[:]:
                        if trade.ticket == order.ticket and trade.trade_id == trade_id:
                            trade.trade_id = ""
                            self.trade_repository.remove_from_pool(trade)
                            self.remove_trade_from_redis(trade)
                    break

            if not success:
                for trade in self.trade_repository.pool[:]:
                    if (trade.trade_id == trade_id and
                            trade.user_id == user_id and
                            trade.order_type in ["BUY_LIMIT", "SELL_LIMIT", "BUY_STOP", "SELL_STOP"]):
                        trade.trade_id = ""
                        self.trade_repository.remove_from_pool(trade)
                        self.remove_trade_from_redis(trade)
                        success = True
                        close_reason = "CANCELED"
                        break
            if not success:
                close_reason = "INVALID_TICKET" if close_reason == "" else close_reason

        response = self.trade_factory.create_close_trade_response(
            trade_id, user_id, account_type, "SUCCESS" if success else "FAILED 17", close_price, close_reason, profit=profit
        )
        try:
            await ws.send(json.dumps(response.model_dump()))
        except ConnectionClosed:
            self.trade_repository.queue_message(
                json.dumps(response.model_dump()))
        except Exception as e:
            self.trade_repository.queue_message(
                json.dumps(response.model_dump()))

    async def stream_orders(self, user_id: str, account_type: str, ws, interval: float = 1.0):
        while True:
            try:
                open_orders = []
                order_ids = set()

                positions = self.mt5_client.get_positions()
                for position in positions:
                    trade_id = str(position.magic)
                    if trade_id in order_ids:
                        continue
                    if position.comment == "TradeMatch" and self.trade_repository.is_user_trade(user_id, trade_id, account_type):
                        open_orders.append({
                            "id": trade_id,
                            "symbol": position.symbol,
                            "trade_type": "BUY" if position.type == mt5.POSITION_TYPE_BUY else "SELL",
                            "order_type": "MARKET",
                            "volume": position.volume,
                            "entry_price": position.price_open,
                            "stop_loss": position.sl,
                            "take_profit": position.tp,
                            "open_time": position.time,
                            "status": "OPEN",
                            "account_type": account_type,
                            "profit": position.profit
                        })
                        order_ids.add(trade_id)

                try:
                    orders = self.mt5_client.get_orders()
                    logger.debug(f"User {user_id}: Retrieved {len(orders)} MT5 orders")
                except mt5.LastError as mt5_err:
                    logger.error(f"MT5 get_orders error for user {user_id}: {str(mt5_err)}")
                    orders = []
                for order in orders:
                    trade_id = str(order.magic)
                    if trade_id in order_ids:
                        continue
                    if order.comment == "Pending" and self.trade_repository.is_user_trade(user_id, trade_id, account_type):
                        order_type = {
                            mt5.ORDER_TYPE_BUY_LIMIT: "BUY_LIMIT",
                            mt5.ORDER_TYPE_SELL_LIMIT: "SELL_LIMIT",
                            mt5.ORDER_TYPE_BUY_STOP: "BUY_STOP",
                            mt5.ORDER_TYPE_SELL_STOP: "SELL_STOP"
                        }.get(order.type, "")
                        if order_type:
                            open_orders.append({
                                "id": trade_id,
                                "symbol": order.symbol,
                                "trade_type": order_type,
                                "order_type": order_type,
                                "volume": order.volume_current,
                                "entry_price": order.price_open,
                                "stop_loss": order.sl,
                                "take_profit": order.tp,
                                "open_time": order.time_setup,
                                "status": "PENDING",
                                "account_type": account_type,
                                "profit": 0.0
                            })
                            order_ids.add(trade_id)

                for trade in self.trade_repository.pool:
                    if not trade.trade_id or not trade.trade_id.strip():
                        logger.warning(f"Invalid trade_id for user {user_id}")
                        continue
                    if (
                        trade.user_id == user_id
                        and trade.account_type == account_type
                        and trade.trade_id not in order_ids
                        and trade.status == "PENDING"
                    ):
                        trade_type = trade.order_type if trade.order_type in ["BUY_LIMIT", "SELL_LIMIT", "BUY_STOP", "SELL_STOP"] else trade.trade_type
                        open_orders.append({
                            "id": trade.trade_id,
                            "symbol": trade.symbol,
                            "trade_type": trade_type,
                            "order_type": trade.order_type,
                            "volume": trade.volume,
                            "entry_price": trade.entry_price,
                            "stop_loss": trade.stop_loss,
                            "take_profit": trade.take_profit,
                            "open_time": int(trade.created_at.timestamp()) if trade.created_at else int(time.time()),
                            "status": "PENDING",
                            "account_type": trade.account_type,
                            "profit": 0.0
                        })
                        order_ids.add(trade.trade_id)

                if open_orders:
                    response = self.trade_factory.create_order_stream_response(user_id, account_type, open_orders)
                    logger.debug(f"User {user_id}: Sending {len(open_orders)} orders: {open_orders}")
                    await ws.send(json.dumps(response.model_dump()))
                else:
                    logger.debug(f"No orders found for user {user_id}, account {account_type}")

                await asyncio.sleep(interval)

            except ConnectionClosed:
                logger.info(f"WebSocket connection closed for user {user_id}")
                break
            except Exception as e:
                logger.error(f"Unexpected error in stream_orders for user {user_id}: {str(e)}")
                await asyncio.sleep(1)
                continue

    async def handle_order_stream_request(self, json_data: dict, ws):
        user_id = json_data.get("user_id", "")
        account_type = json_data.get("account_type", "")
        open_orders = []
        positions = self.mt5_client.get_positions()
        for position in positions:
            trade_id = str(position.magic)
            if position.comment == "Trade" and self.trade_repository.is_user_trade(user_id, trade_id, account_type):
                open_orders.append({
                    "id": trade_id,
                    "symbol": position.symbol,
                    "trade_type": "BUY" if position.type == mt5.POSITION_TYPE_BUY else "SELL",
                    "order_type": "MARKET",
                    "volume": position.volume,
                    "entry_price": position.price_open,
                    "stop_loss": position.sl,
                    "take_profit": position.tp,
                    "open_time": position.time,
                    "status": "OPEN",
                    "account_type": account_type
                })
        orders = self.mt5_client.get_orders()
        for order in orders:
            trade_id = str(order.magic)
            if order.comment == "Pending" and self.trade_repository.is_user_trade(user_id, trade_id, account_type):
                order_type = {
                    mt5.ORDER_TYPE_BUY_LIMIT: "BUY_LIMIT",
                    mt5.ORDER_TYPE_SELL_LIMIT: "SELL_LIMIT",
                    mt5.ORDER_TYPE_BUY_STOP: "BUY_STOP",
                    mt5.ORDER_TYPE_SELL_STOP: "SELL_STOP"
                }.get(order.type, "")
                open_orders.append({
                    "id": trade_id,
                    "symbol": order.symbol,
                    "trade_type": order_type,
                    "order_type": order_type,
                    "volume": order.volume_current,
                    "entry_price": order.price_open,
                    "stop_loss": order.sl,
                    "take_profit": order.tp,
                    "open_time": order.time_setup,
                    "status": "PENDING",
                    "account_type": account_type
                })
        response = self.trade_factory.create_order_stream_response(
            user_id, account_type, open_orders)
        try:
            await ws.send(json.dumps(response.model_dump()))
        except ConnectionClosed:
            self.trade_repository.queue_message(
                json.dumps(response.model_dump()))
        except Exception as e:
            self.trade_repository.queue_message(
                json.dumps(response.model_dump()))

    async def handle_modify_trade_request(self, json_data: dict, ws):
        trade_id = json_data.get("trade_id", "")
        trade_code = json_data.get("trade_code", "")
        user_id = json_data.get("user_id", "")
        account_type = json_data.get("account_type", "")
        new_price = json_data.get("entry_price", 0.0)
        new_volume = json_data.get("volume", 0.0)
        for trade in self.trade_repository.pool:
            if trade.trade_id == trade_id and trade.user_id == user_id and trade.account_type == account_type:
                if trade.order_type == "MARKET":
                    await self.send_trade_response(trade_id, trade_code, user_id, "FAILED 18", "", ws, error="Cannot modify MARKET orders")
                    return
                if new_price > 0:
                    trade.entry_price = new_price
                if new_volume > 0:
                    symbol_info = self.mt5_client.get_symbol_info(trade.symbol)
                    if new_volume < symbol_info.volume_min or new_volume > symbol_info.volume_max:
                        await self.send_trade_response(trade_id, trade_code, user_id, "FAILED 19", "", ws, error="Invalid volume")
                        return
                    trade.volume = new_volume
                self.save_trade_to_redis(trade)
                await self.send_trade_response(trade_id, trade_code, user_id, "MODIFIED", "", ws)
                return
        await self.send_trade_response(trade_id, trade_code, user_id, "FAILED 20", "", ws, error="Trade not found")

    def process_tick(self):
        current_time = int(self.get_timestamp())
        for trade in self.trade_repository.pool:
            if trade.trade_id == "" or trade.ticket == 0:
                continue
            if trade.order_type in ["BUY_STOP", "SELL_STOP"]:
                market_price = self.mt5_client.get_symbol_tick(
                    trade.symbol).bid
                triggered = (trade.order_type == "BUY_STOP" and market_price >= trade.entry_price) or \
                    (trade.order_type == "SELL_STOP" and market_price <= trade.entry_price)
                if triggered:
                    strategy = self.strategies.get("MARKET")
                    if strategy.execute(trade, self.mt5_client):
                        response = self.trade_factory.create_trade_response(
                            trade.trade_id, trade.trade_code, trade.user_id, "EXECUTED", "")
                        self.trade_repository.queue_message(
                            json.dumps(response.model_dump()))
                        self.remove_trade_from_redis(trade)
            if trade.expiration > 0 and trade.expiration <= current_time:
                trade.trade_id = ""
                response = self.trade_factory.create_trade_response(
                    trade.trade_id, trade.trade_code, trade.user_id, "EXPIRED", "")
                self.trade_repository.queue_message(
                    json.dumps(response.model_dump()))
                self.trade_repository.remove_from_pool(trade)
                self.remove_trade_from_redis(trade)