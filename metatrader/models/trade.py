from pydantic import BaseModel
from datetime import datetime
from typing import Optional, List
import time


class PoolTrade(BaseModel):
    trade_id: str
    user_id: str
    symbol: str
    trade_code: int
    trade_type: str
    order_type: str
    account_type: str
    account_name: str
    leverage: int
    volume: float
    entry_price: float
    stop_loss: float
    take_profit: float
    timestamp: int
    comment: str = ""
    slippage: int = 0
    expiration: int
    magic: int = int(time.time() % 1000000)
    ticket: int = 0
    created_at: Optional[datetime] = None
    profit: float = 0.0
    status: str = "PENDING"


class TradeResponse(BaseModel):
    type: str = "trade_response"
    trade_id: str
    trade_retcode: int
    user_id: str
    status: str
    matched_trade_id: str
    matched_volume: float
    timestamp: float


class CloseTradeResponse(BaseModel):
    type: str = "close_trade_response"
    trade_id: str
    user_id: str
    account_type: str
    account_name: str
    status: str
    close_price: float
    close_reason: str
    timestamp: float


class OrderStreamResponse(BaseModel):
    type: str = "order_stream_response"
    user_id: str
    account_type: str
    account_name: str
    trades: List[dict]
    timestamp: float


class BalanceResponse(BaseModel):
    type: str = "balance_response"
    user_id: str
    account_name: str
    account_type: str
    balance: float
    error: Optional[str] = None
    timestamp: float
