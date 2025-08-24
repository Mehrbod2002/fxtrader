from typing import Dict, List
from models.trade import PoolTrade
from config.settings import settings
from utils.logger import logger
from services.mt5_client import MT5Client
from factories.trade_factory import TradeFactory
import time
import redis
import json
import asyncio


class TradeRepository:
    def __init__(self, mt5_client: MT5Client):
        self.mt5_client = mt5_client
        self.trade_factory = TradeFactory(mt5_client)
        self.pool: List[PoolTrade] = []
        self.pool_size: int = 0
        self.redis_client = redis.Redis(
            host=settings.REDIS_HOST, port=settings.REDIS_PORT, db=0)
        self.user_balances: Dict[str, float] = {}
        self.message_queue = []

    def add_to_pool(self, trade: PoolTrade):
        self.pool.append(trade)
        self.pool_size += 1

    def remove_from_pool(self, trade: PoolTrade):
        if trade in self.pool:
            self.pool.remove(trade)
            self.pool_size -= 1

    def queue_message(self, message: str):
        self.redis_client.lpush("message_queue", message)
        if self.redis_client.llen("message_queue") > 1000:
            self.redis_client.rpop("message_queue")

    def cleanup_trade_pool(self):
        current_time = int(time.time())
        expired_trades = [trade for trade in self.pool if trade.trade_id !=
                          "" and trade.expiration > 0 and trade.expiration <= current_time]
        self.pool = [trade for trade in self.pool if trade.trade_id == "" or (
            trade.expiration == 0 or trade.expiration > current_time)]
        self.pool_size = len(self.pool)
        for trade in expired_trades:
            response = {
                "type": "trade_response",
                "trade_id": trade.trade_id,
                "user_id": trade.user_id,
                "status": "EXPIRED",
                "matched_trade_id": "",
            }
            self.queue_message(json.dumps(response))

    def find_matching_trade(self, new_trade: PoolTrade) -> int:
        self.cleanup_trade_pool()

        best_match_index = -1
        oldest_time = float('inf')

        for i, trade in enumerate(self.pool):
            if trade.trade_id == "":
                continue

            if self.can_match_trades(new_trade, trade):
                if trade.timestamp < oldest_time:
                    oldest_time = trade.timestamp
                    best_match_index = i

        return best_match_index

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

    def can_match_trades(self, trade1: PoolTrade, trade2: PoolTrade) -> bool:
        if (trade1.trade_type == trade2.trade_type or trade1.symbol != trade2.symbol or
                trade1.account_type != trade2.account_type):
            return False

        symbol_info = self.mt5_client.get_symbol_info(trade1.symbol)
        if not symbol_info:
            print(f"Failed to get symbol info for {trade1.symbol}")
            return False

        tick_size = getattr(symbol_info, 'trade_tick_size', 0.01)  # Default to 0.01 for BTCUSD
        try:
            stops_level = symbol_info.stops_level
        except AttributeError:
            print(f"Warning: stops_level undefined for {trade1.symbol}. Using default value of 0.")
            stops_level = 0  # Default to 0 if undefined
        min_sl_distance = stops_level * tick_size
        digits = getattr(symbol_info, 'digits', 2)  # Default to 2 decimal places for BTCUSD

        def round_price(price: float) -> float:
            return round(price / tick_size) * tick_size

        def validate_sl_tp(trade: PoolTrade) -> bool:
            """Validate SL and TP against symbol constraints."""
            entry_price = round_price(trade.entry_price)
            sl = round_price(trade.stop_loss) if trade.stop_loss else None
            tp = round_price(trade.take_profit) if trade.take_profit else None

            if stops_level == 0:
                print(f"Warning: No stops_level defined for {trade1.symbol}. Skipping SL/TP validation.")
                return True

            if sl:
                sl_distance = abs(entry_price - sl)
                if sl_distance < min_sl_distance:
                    print(f"Invalid SL for {trade.trade_id}: {sl_distance} < {min_sl_distance}")
                    return False
            if tp:
                tp_distance = abs(entry_price - tp)
                if tp_distance < min_sl_distance:
                    print(f"Invalid TP for {trade.trade_id}: {tp_distance} < {min_sl_distance}")
                    return False
            return True

        # Validate SL/TP for both trades
        if not (validate_sl_tp(trade1) and validate_sl_tp(trade2)):
            return False

        def get_market_price():
            try:
                tick = self.mt5_client.get_symbol_tick(trade1.symbol)
                return tick.bid if tick else None
            except:
                return None

        print("matching: ", trade1, trade2)
        if trade1.order_type == "MARKET" or trade2.order_type == "MARKET":
            return True

        limit_types = ["BUY_LIMIT", "SELL_LIMIT"]
        stop_types = ["BUY_STOP", "SELL_STOP"]

        if trade1.order_type in limit_types and trade2.order_type in limit_types:
            rounded_price1 = round_price(trade1.entry_price)
            rounded_price2 = round_price(trade2.entry_price)

            if trade1.order_type == "BUY_LIMIT" and trade2.order_type == "SELL_LIMIT":
                return rounded_price1 >= rounded_price2
            elif trade1.order_type == "SELL_LIMIT" and trade2.order_type == "BUY_LIMIT":
                return rounded_price1 <= rounded_price2

        elif trade1.order_type in stop_types and trade2.order_type in limit_types:
            rounded_price1 = round_price(trade1.entry_price)
            rounded_price2 = round_price(trade2.entry_price)

            if trade1.order_type == "BUY_STOP" and trade2.order_type == "SELL_LIMIT":
                return rounded_price1 >= rounded_price2
            elif trade1.order_type == "SELL_STOP" and trade2.order_type == "BUY_LIMIT":
                return rounded_price1 <= rounded_price2

        elif trade2.order_type in stop_types and trade1.order_type in limit_types:
            rounded_price1 = round_price(trade1.entry_price)
            rounded_price2 = round_price(trade2.entry_price)

            if trade1.order_type == "BUY_LIMIT" and trade2.order_type == "SELL_STOP":
                return rounded_price1 >= rounded_price2
            elif trade1.order_type == "SELL_LIMIT" and trade2.order_type == "BUY_STOP":
                return rounded_price1 <= rounded_price2

        elif (trade1.order_type in stop_types and trade2.order_type == "MARKET") or \
                (trade2.order_type in stop_types and trade1.order_type == "MARKET"):
            market_price = get_market_price()
            if market_price is None:
                return False

            rounded_market = round_price(market_price)

            if trade1.order_type in stop_types:
                stop_price = round_price(trade1.entry_price)
                if trade1.order_type == "BUY_STOP":
                    return rounded_market >= stop_price
                else:
                    return rounded_market <= stop_price
            else:
                stop_price = round_price(trade2.entry_price)
                if trade2.order_type == "BUY_STOP":
                    return rounded_market >= stop_price
                else:
                    return rounded_market <= stop_price

        elif trade1.order_type in stop_types and trade2.order_type in stop_types:
            market_price = get_market_price()
            if market_price is None:
                return False

            rounded_market = round_price(market_price)
            rounded_price1 = round_price(trade1.entry_price)
            rounded_price2 = round_price(trade2.entry_price)

            if trade1.order_type == "BUY_STOP" and trade2.order_type == "SELL_STOP":
                return rounded_market >= rounded_price1 and rounded_market <= rounded_price2
            elif trade1.order_type == "SELL_STOP" and trade2.order_type == "BUY_STOP":
                return rounded_market <= rounded_price1 and rounded_market >= rounded_price2

        return False

    def is_user_trade(self, user_id: str, trade_id: str, account_type: str) -> bool:
        for trade in self.pool:
            if (
                trade.user_id == user_id
                and trade.account_type == account_type
                and (trade.trade_id == trade_id or str(trade.magic) == trade_id)
            ):
                return True
        return False

    def get_timestamp(self):
        tick = self.mt5_client.get_symbol_tick(settings.SYMBOL)
        return tick.time if tick else time.time()

    async def get_user_balance(self, user_id: str, account_type: str, account_name: str, ws) -> float:
        balance_request = {
            "type": "balance_request",
            "user_id": user_id,
            "account_name": account_name,
            "account_type": account_type,
            "timestamp": float(self.get_timestamp())
        }
        try:
            await ws.send(json.dumps(balance_request))

            async with asyncio.timeout(10):
                message = await ws.recv()
                response = json.loads(message)
                if response.get("type") == "balance_response" and response.get("user_id") == user_id:
                    return float(response.get("balance", 0.0))
                else:
                    logger.error(f"Invalid balance response: {response}")
                    return 0.0
        except asyncio.TimeoutError:
            return 0.0
        except Exception as e:
            return 0.0

    def save_trade_to_redis(self, trade: PoolTrade):
        try:
            trade_key = f"trade:{trade.trade_id}:{trade.user_id}:{trade.account_type}"
            trade_data = trade.__dict__.copy()
            trade_data["created_at"] = trade_data["created_at"].isoformat() if trade_data["created_at"] else None
            self.redis_client.set(trade_key, json.dumps(trade_data))
            logger.info(f"Saved trade {trade.trade_id} to Redis")
        except Exception as e:
            logger.error(f"Error saving trade {trade.trade_id} to Redis: {str(e)}")