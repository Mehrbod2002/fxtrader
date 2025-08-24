from models.trade import PoolTrade, TradeResponse, CloseTradeResponse, OrderStreamResponse, BalanceResponse
from uuid import uuid4
from config.settings import settings
from services.mt5_client import MT5Client
from typing import List
from datetime import datetime


class TradeFactory:
    def __init__(self, mt5_client: MT5Client):
        self.mt5_client = mt5_client

    def create_trade(self, json_data: dict) -> PoolTrade:
        return PoolTrade(
            trade_id=json_data.get("trade_id", str(uuid4())),
            user_id=json_data.get("user_id", ""),
            symbol=json_data.get("symbol", ""),
            account_name=json_data.get("account_name", ""),
            trade_type=json_data.get("trade_type", ""),
            order_type=json_data.get("order_type", ""),
            account_type=json_data.get("account_type", ""),
            leverage=int(json_data.get("leverage", 0)),
            volume=json_data.get("volume", 0.0),
            entry_price=json_data.get("entry_price", 0.0),
            stop_loss=json_data.get("stop_loss", 0.0),
            take_profit=json_data.get("take_profit", 0.0),
            timestamp=int(json_data.get(
                "timestamp", self.mt5_client.get_symbol_tick(settings.SYMBOL).time)),
            expiration=int(json_data.get("expiration", 0)),
            created_at=datetime.now()
        )

    def create_trade_response(self, trade_id: str, user_id: str, status: str, matched_volume: float, matched_trade_id: str, remaining_volume: float = 0) -> TradeResponse:
        return TradeResponse(
            trade_id=trade_id,
            user_id=user_id,
            status=status,
            matched_volume=matched_volume,
            matched_trade_id=matched_trade_id,
            timestamp=float(
                self.mt5_client.get_symbol_tick(settings.SYMBOL).time)
        )

    def create_close_trade_response(self, trade_id: str, user_id: str, account_type: str, status: str,
                                    close_price: float, close_reason: str, profit: float = 0) -> CloseTradeResponse:
        return CloseTradeResponse(
            trade_id=trade_id,
            user_id=user_id,
            account_type=account_type,
            status=status,
            close_price=close_price,
            close_reason=close_reason,
            profit=profit,
            timestamp=float(
                self.mt5_client.get_symbol_tick(settings.SYMBOL).time)
        )

    def create_balance_response(self, user_id: str, account_type: str, balance: float, error: str = None) -> BalanceResponse:
        return BalanceResponse(
            user_id=user_id,
            account_type=account_type,
            balance=balance,
            error=error,
            timestamp=float(
                self.mt5_client.get_symbol_tick(settings.SYMBOL).time)
        )

    def create_order_stream_response(self, user_id: str, account_type: str, trades: List[dict]) -> OrderStreamResponse:
        return OrderStreamResponse(
            user_id=user_id,
            account_type=account_type,
            trades=trades,
            timestamp=float(
                self.mt5_client.get_symbol_tick(settings.SYMBOL).time)
        )
