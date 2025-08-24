from abc import ABC, abstractmethod
from models.trade import PoolTrade
from services.mt5_client import MT5Client


class TradeStrategy(ABC):
    @abstractmethod
    def execute(self, trade: PoolTrade, mt5_client: MT5Client) -> tuple[bool, int]:
        pass


class MarketTradeStrategy(TradeStrategy):
    def execute(self, trade: PoolTrade, mt5_client: MT5Client) -> tuple[bool, int]:
        price = mt5_client.get_symbol_tick(
            trade.symbol).ask if trade.trade_type == "BUY" else mt5_client.get_symbol_tick(trade.symbol).bid
        return mt5_client.execute_market_trade(trade, price)


class PendingTradeStrategy(TradeStrategy):
    def execute(self, trade: PoolTrade, mt5_client: MT5Client) -> tuple[bool, int]:
        return mt5_client.place_pending_order(trade)
