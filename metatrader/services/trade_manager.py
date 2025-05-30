import json
import time
from websockets.exceptions import ConnectionClosed
from models.trade import PoolTrade
from services.mt5_client import MT5Client
from factories.trade_factory import TradeFactory
from repositories.trade_repository import TradeRepository
from strategies.trade_strategy import MarketTradeStrategy, PendingTradeStrategy
from config.settings import settings
from utils.logger import logger

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

    def get_timestamp(self):
        tick = self.mt5_client.get_symbol_tick(settings.SYMBOL)
        return tick.time if tick else time.time()

    def validate_trade(self, trade: PoolTrade) -> bool:
        if trade.trade_type not in ["BUY", "SELL"]:
            return False
        if trade.order_type not in ["MARKET", "LIMIT", "BUY_LIMIT", "SELL_LIMIT", "BUY_STOP", "SELL_STOP"]:
            return False
        if trade.account_type not in ["DEMO", "REAL"]:
            return False
        if trade.leverage <= 0 or trade.volume <= 0:
            return False
        if trade.order_type != "MARKET" and trade.entry_price <= 0:
            return False
        if trade.order_type == "MARKET" and trade.entry_price > 0:
            return False
        if trade.stop_loss < 0 or trade.take_profit < 0:
            return False
        current_time = int(self.get_timestamp())
        if trade.expiration > 0 and trade.expiration <= current_time:
            return False
        return True

    async def handle_trade_request(self, json_data: dict, ws) -> bool:
        symbol = json_data.get("symbol", "")
        if not self.mt5_client.get_symbol_info(symbol):
            await self.send_trade_response(json_data.get("trade_id", ""), json_data.get("user_id", ""),
                                          "FAILED", "", ws, error="Invalid symbol")
            return False

        volume = json_data.get("volume", 0.0)
        symbol_info = self.mt5_client.get_symbol_info(symbol)
        if volume < symbol_info.volume_min or volume > symbol_info.volume_max:
            await self.send_trade_response(json_data.get("trade_id", ""), json_data.get("user_id", ""),
                                          "FAILED", "", ws, error="Invalid volume")
            return False

        trade = self.trade_factory.create_trade(json_data)
        print("trade requested: ",trade, "\n\n")
        if not self.validate_trade(trade):
            await self.send_trade_response(trade.trade_id, trade.user_id, "FAILED", "", ws, error="Invalid trade parameters")
            print("self trade")            
            return False

        match_index = self.trade_repository.find_matching_trade(trade)
        if match_index >= 0:
            await self.execute_matched_trades(trade, self.trade_repository.pool[match_index], ws)
            if trade.trade_id != "":
                self.trade_repository.add_to_pool(trade)
                await self.send_trade_response(trade.trade_id, trade.user_id, "PENDING", "", ws)
        else:
            strategy = self.strategies.get(trade.order_type)
            print("not matched")
            if strategy and strategy.execute(trade, self.mt5_client) and trade.trade_id != "":
                self.trade_repository.add_to_pool(trade)
                await self.send_trade_response(trade.trade_id, trade.user_id, "PENDING", "", ws)
            else:
                await self.send_trade_response(trade.trade_id, trade.user_id, "FAILED", "", ws, error="Failed to execute trade")
        return True

    async def execute_matched_trades(self, trade1: PoolTrade, trade2: PoolTrade, ws):
        match_price = self.mt5_client.get_symbol_tick(trade1.symbol).bid if trade1.order_type == "MARKET" or trade2.order_type == "MARKET" \
            else (trade1.entry_price + trade2.entry_price) / 2.0
        success = True
        ticket1, ticket2 = 0, 0
        error = ""

        request1 = {
            "action": mt5.TRADE_ACTION_DEAL,
            "symbol": trade1.symbol,
            "volume": trade1.volume,
            "type": mt5.ORDER_TYPE_BUY if trade1.trade_type == "BUY" else mt5.ORDER_TYPE_SELL,
            "price": match_price,
            "sl": trade1.stop_loss,
            "tp": trade1.take_profit,
            "comment": "TradeMatch",
            "type_time": mt5.ORDER_TIME_GTC,
            "type_filling": mt5.ORDER_FILLING_IOC,
            "magic": int(trade1.trade_id.replace("-", ""))
        }
        request2 = {
            "action": mt5.TRADE_ACTION_DEAL,
            "symbol": trade2.symbol,
            "volume": trade2.volume,
            "type": mt5.ORDER_TYPE_BUY if trade2.trade_type == "BUY" else mt5.ORDER_TYPE_SELL,
            "price": match_price,
            "sl": trade2.stop_loss,
            "tp": trade2.take_profit,
            "comment": "TradeMatch",
            "type_time": mt5.ORDER_TIME_GTC,
            "type_filling": mt5.ORDER_FILLING_IOC,
            "magic": int(trade2.trade_id.replace("-", ""))
        }
        result1 = mt5.order_send(request1)
        if result1.retcode == mt5.TRADE_RETCODE_DONE:
            ticket1 = result1.order
        else:
            success = False
            error = result1.comment
        result2 = mt5.order_send(request2)
        if result2.retcode == mt5.TRADE_RETCODE_DONE:
            ticket2 = result2.order
        else:
            success = False
            error = result2.comment if error == "" else error

        status = "MATCHED" if success else "FAILED"
        await self.send_trade_response(trade1.trade_id, trade1.user_id, status, trade2.trade_id, ws, error=error)
        await self.send_trade_response(trade2.trade_id, trade2.user_id, status, trade1.trade_id, ws, error=error)

        if success:
            for trade in self.trade_repository.pool:
                if trade.trade_id == trade2.trade_id and trade.ticket > 0:
                    self.mt5_client.close_order(trade.ticket, trade.symbol, trade.volume,
                                               mt5.POSITION_TYPE_BUY if trade.trade_type == "BUY" else mt5.POSITION_TYPE_SELL)
                    trade.trade_id = ""
                    break
            logger.info(f"Matched trades: {trade1.trade_id} and {trade2.trade_id} at price: {match_price}")

    async def send_trade_response(self, trade_id: str, user_id: str, status: str, matched_trade_id: str, ws, error: str = None):
        response = self.trade_factory.create_trade_response(trade_id, user_id, status, matched_trade_id)
        if error:
            response.status = "FAILED"
        try:
            await ws.send(json.dumps(response.dict()))
            logger.info(f"Sent trade response: {response.dict()}")
        except ConnectionClosed:
            logger.error(f"WebSocket closed while sending trade response: {response.dict()}")
            self.trade_repository.queue_message(json.dumps(response.dict()))
        except Exception as e:
            logger.error(f"Failed to send trade response: {response.dict()}. Error: {str(e)}")
            self.trade_repository.queue_message(json.dumps(response.dict()))

    async def handle_balance_request(self, json_data: dict, ws):
        user_id = json_data.get("user_id", "")
        account_type = json_data.get("account_type", "")
        balance = self.mt5_client.get_balance()
        response = self.trade_factory.create_balance_response(user_id, account_type, balance)
        try:
            await ws.send(json.dumps(response.dict()))
            logger.info(f"Sent balance response: {response.dict()}")
        except ConnectionClosed:
            logger.error(f"WebSocket closed while sending balance response: {response.dict()}")
            self.trade_repository.queue_message(json.dumps(response.dict()))
        except Exception as e:
            logger.error(f"Failed to send balance response: {response.dict()}. Error: {str(e)}")
            self.trade_repository.queue_message(json.dumps(response.dict()))

    async def handle_close_trade_request(self, json_data: dict, ws):
        user_id = json_data.get("user_id", "")
        trade_id = json_data.get("trade_id", "")
        account_type = json_data.get("account_type", "")
        success = False
        close_price = 0.0
        close_reason = ""
        positions = self.mt5_client.get_positions()
        for position in positions:
            if position.comment == "Trade" and self.trade_repository.is_user_trade(user_id, trade_id, account_type):
                close_price = self.mt5_client.get_symbol_tick(position.symbol).bid if position.type == mt5.POSITION_TYPE_BUY else self.mt5_client.get_symbol_tick(position.symbol).ask
                success = self.mt5_client.close_order(position.ticket, position.symbol, position.volume, position.type)
                close_reason = "CLOSED" if success else "FAILED"
                break
        else:
            orders = self.mt5_client.get_orders()
            for order in orders:
                if order.comment == "Pending" and self.trade_repository.is_user_trade(user_id, trade_id, account_type):
                    success = self.mt5_client.close_order(order.ticket, order.symbol, order.volume_current, order.type)
                    close_reason = "CANCELED" if success else "FAILED"
                    for trade in self.trade_repository.pool:
                        if trade.ticket == order.ticket and trade.trade_id == trade_id:
                            trade.trade_id = ""
                            break
                    break
            else:
                close_reason = "INVALID_TICKET"
        response = self.trade_factory.create_close_trade_response(trade_id, user_id, account_type,
                                                                "SUCCESS" if success else "FAILED",
                                                                close_price, close_reason)
        try:
            await ws.send(json.dumps(response.dict()))
            logger.info(f"Sent close trade response: {response.dict()}")
        except ConnectionClosed:
            logger.error(f"WebSocket closed while sending close trade response: {response.dict()}")
            self.trade_repository.queue_message(json.dumps(response.dict()))
        except Exception as e:
            logger.error(f"Failed to send close trade response: {response.dict()}. Error: {str(e)}")
            self.trade_repository.queue_message(json.dumps(response.dict()))

    async def handle_order_stream_request(self, json_data: dict, ws):
        user_id = json_data.get("user_id", "")
        account_type = json_data.get("account_type", "")
        open_orders = []
        positions = self.mt5_client.get_positions()
        for position in positions:
            if position.comment == "Trade" and self.trade_repository.is_user_trade(user_id, trade_id, account_type):
                open_orders.append({
                    "id": position.magic,
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
            if order.comment == "Pending" and self.trade_repository.is_user_trade(user_id, trade_id, account_type):
                order_type = {
                    mt5.ORDER_TYPE_BUY_LIMIT: "BUY_LIMIT",
                    mt5.ORDER_TYPE_SELL_LIMIT: "SELL_LIMIT",
                    mt5.ORDER_TYPE_BUY_STOP: "BUY_STOP",
                    mt5.ORDER_TYPE_SELL_STOP: "SELL_STOP"
                }.get(order.type, "")
                open_orders.append({
                    "id": order.magic,
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
        response = self.trade_factory.create_order_stream_response(user_id, account_type, open_orders)
        try:
            await ws.send(json.dumps(response.dict()))
            logger.info(f"Sent order stream response: {response.dict()}")
        except ConnectionClosed:
            logger.error(f"WebSocket closed while sending order stream response: {response.dict()}")
            self.trade_repository.queue_message(json.dumps(response.dict()))
        except Exception as e:
            logger.error(f"Failed to send order stream response: {response.dict()}. Error: {str(e)}")
            self.trade_repository.queue_message(json.dumps(response.dict()))

    def process_tick(self):
        current_time = int(self.get_timestamp())
        for trade in self.trade_repository.pool:
            if trade.trade_id == "" or trade.ticket == 0:
                continue
            if trade.order_type in ["BUY_STOP", "SELL_STOP"]:
                market_price = self.mt5_client.get_symbol_tick(trade.symbol).bid
                triggered = (trade.order_type == "BUY_STOP" and market_price >= trade.entry_price) or \
                           (trade.order_type == "SELL_STOP" and market_price <= trade.entry_price)
                if triggered:
                    strategy = self.strategies.get("MARKET")
                    strategy.execute(trade, self.mt5_client)
            if trade.expiration > 0 and trade.expiration <= current_time:
                trade.trade_id = ""
                response = self.trade_factory.create_trade_response(trade.trade_id, trade.user_id, "EXPIRED", "")
                self.trade_repository.queue_message(json.dumps(response.dict()))