from typing import List
from models.trade import PoolTrade
from config.settings import settings
from utils.logger import logger
from services.mt5_client import MT5Client

class TradeRepository:
    def __init__(self, mt5_client: MT5Client):
        self.mt5_client = mt5_client
        self.pool: List[PoolTrade] = []
        self.pool_size: int = 0
        self.message_queue: List[str] = []

    def add_to_pool(self, trade: PoolTrade):
        self.pool.append(trade)
        self.pool_size += 1
        logger.info(f"Added trade to pool: {trade.trade_id}, Symbol: {trade.symbol}, Type: {trade.trade_type}")

    def cleanup_trade_pool(self):
        self.pool = [trade for trade in self.pool if trade.trade_id != ""]
        self.pool_size = len(self.pool)

    def find_matching_trade(self, new_trade: PoolTrade) -> int:
        for i, trade in enumerate(self.pool):
            if trade.trade_id == "" or trade.ticket == 0:
                continue
            if self.can_match_trades(new_trade, trade):
                return i
        return -1

    def can_match_trades(self, trade1: PoolTrade, trade2: PoolTrade) -> bool:
        if (trade1.trade_type == trade2.trade_type or trade1.symbol != trade2.symbol or
                trade1.volume != trade2.volume or trade1.account_type != trade2.account_type):
            return False
        if trade1.order_type == "MARKET" or trade2.order_type == "MARKET":
            return True
        if trade1.order_type in ["BUY_LIMIT", "SELL_LIMIT"] and trade2.order_type in ["BUY_LIMIT", "SELL_LIMIT"]:
            return abs(trade1.entry_price - trade2.entry_price) <= settings.SPREAD_TOLERANCE
        return False

    def is_user_trade(self, user_id: str, trade_id: str, account_type: str) -> bool:
        for trade in self.pool:
            if trade.trade_id == trade_id and trade.user_id == user_id and trade.account_type == account_type:
                return True
        return False

    def queue_message(self, message: str):
        if len(self.message_queue) > 1000:
            self.message_queue.pop(0)
        self.message_queue.append(message)
        logger.info(f"Queued message: {message}")