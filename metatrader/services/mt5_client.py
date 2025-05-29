import MetaTrader5 as mt5
from config.settings import settings
from utils.logger import logger

class MT5Client:
    def __init__(self):
        self._initialize()

    def _initialize(self):
        if not mt5.initialize(
            path=settings.MT5_PATH,
            login=settings.MT5_LOGIN,
            password=settings.MT5_PASSWORD,
            server=settings.MT5_SERVER,
            timeout=60000
        ):
            logger.error(f"Failed to initialize MetaTrader5: {mt5.last_error()}")
            raise SystemExit
        if not mt5.symbol_select(settings.SYMBOL, True):
            logger.error(f"Failed to select symbol {settings.SYMBOL}: {mt5.last_error()}")
            mt5.shutdown()
            raise SystemExit

    def execute_market_trade(self, trade, price: float) -> bool:
        request = {
            "action": mt5.TRADE_ACTION_DEAL,
            "symbol": trade.symbol,
            "volume": trade.volume,
            "type": mt5.ORDER_TYPE_BUY if trade.trade_type == "BUY" else mt5.ORDER_TYPE_SELL,
            "price": price,
            "sl": trade.stop_loss,
            "tp": trade.take_profit,
            "comment": "Trade",
            "type_time": mt5.ORDER_TIME_GTC,
            "type_filling": mt5.ORDER_FILLING_IOC,
            "magic": int(trade.trade_id.replace("-", ""))
        }
        result = mt5.order_send(request)
        if result.retcode == mt5.TRADE_RETCODE_DONE:
            trade.ticket = result.order
            logger.info(f"Executed trade: {trade.trade_id}, Symbol: {trade.symbol}, Type: {trade.trade_type}, Price: {price}")
            return True
        logger.error(f"Failed to execute trade: {trade.trade_id}. Error: {result.comment}")
        return False

    def place_pending_order(self, trade) -> bool:
        order_types = {
            "BUY_LIMIT": mt5.ORDER_TYPE_BUY_LIMIT,
            "SELL_LIMIT": mt5.ORDER_TYPE_SELL_LIMIT,
            "BUY_STOP": mt5.ORDER_TYPE_BUY_STOP,
            "SELL_STOP": mt5.ORDER_TYPE_SELL_STOP
        }
        request = {
            "action": mt5.TRADE_ACTION_PENDING,
            "symbol": trade.symbol,
            "volume": trade.volume,
            "type": order_types[trade.order_type],
            "price": trade.entry_price,
            "sl": trade.stop_loss,
            "tp": trade.take_profit,
            "comment": "Pending",
            "type_time": mt5.ORDER_TIME_SPECIFIED if trade.expiration > 0 else mt5.ORDER_TIME_GTC,
            "expiration": trade.expiration if trade.expiration > 0 else 0,
            "type_filling": mt5.ORDER_FILLING_IOC,
            "magic": int(trade.trade_id.replace("-", ""))
        }
        result = mt5.order_send(request)
        if result.retcode == mt5.TRADE_RETCODE_DONE:
            trade.ticket = result.order
            logger.info(f"Placed pending order: {trade.trade_id}, Symbol: {trade.symbol}, Type: {trade.trade_type}")
            return True
        logger.error(f"Failed to place pending order: {trade.trade_id}. Error: {result.comment}")
        return False

    def close_order(self, ticket: int, symbol: str, volume: float, position_type: int) -> bool:
        request = {
            "action": mt5.TRADE_ACTION_DEAL,
            "symbol": symbol,
            "volume": volume,
            "type": mt5.ORDER_TYPE_SELL if position_type == mt5.POSITION_TYPE_BUY else mt5.ORDER_TYPE_BUY,
            "position": ticket,
            "price": mt5.symbol_info_tick(symbol).bid if position_type == mt5.POSITION_TYPE_BUY else mt5.symbol_info_tick(symbol).ask,
            "type_time": mt5.ORDER_TIME_GTC,
            "type_filling": mt5.ORDER_FILLING_IOC
        }
        result = mt5.order_send(request)
        return result.retcode == mt5.TRADE_RETCODE_DONE

    def get_balance(self) -> float:
        return mt5.account_info().balance

    def get_positions(self):
        return mt5.positions_get()

    def get_orders(self):
        return mt5.orders_get()

    def get_symbol_info(self, symbol: str):
        return mt5.symbol_info(symbol)

    def get_symbol_tick(self, symbol: str):
        return mt5.symbol_info_tick(symbol)

    def shutdown(self):
        mt5.shutdown()
        logger.info("MetaTrader5 connection closed")