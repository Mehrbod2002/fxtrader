import MetaTrader5 as mt5
from threading import Lock
from models.trade import PoolTrade
from utils.logger import logger


class MT5Client:
    def __init__(self):
        self.margin_lock = Lock()
        if not mt5.initialize():
            raise Exception("MT5 initialization failed")

    def get_symbol_info(self, symbol):
        symbol_info = mt5.symbol_info(symbol)
        if not symbol_info:
            logger.error(f"Failed to get symbol info for {symbol}")
        return symbol_info

    def get_symbol_tick(self, symbol):
        tick = mt5.symbol_info_tick(symbol)
        if not tick:
            logger.error(f"Failed to get tick data for {symbol}")
        return tick

    def get_account_info(self):
        account_info = mt5.account_info()
        if not account_info:
            logger.error("Failed to get account info")
        return account_info

    def check_margin(self, symbol: str, volume: float, trade_type: str) -> bool:
        with self.margin_lock:
            try:
                symbol_info = self.get_symbol_info(symbol)
                if not symbol_info:
                    return False

                tick = self.get_symbol_tick(symbol)
                if not tick:
                    return False
                market_price = tick.bid if trade_type == "SELL" else tick.ask

                account_info = self.get_account_info()
                if not account_info:
                    return False

                contract_size = symbol_info.trade_contract_size
                leverage = account_info.leverage
                required_margin = (volume * contract_size *
                                   market_price) / leverage

                free_margin = account_info.margin_free
                return free_margin >= required_margin
            except Exception as e:
                return False

    def get_positions(self):
        positions = mt5.positions_get()
        if positions is None:
            logger.error("Failed to get positions")
        return positions if positions is not None else []

    def get_orders(self):
        orders = mt5.orders_get()
        if orders is None:
            logger.error("Failed to get orders")
        return orders if orders is not None else []

    def get_symbol_filling_mode(self, symbol: str) -> int:
        try:
            symbol_info = self.get_symbol_info(symbol)
            if not symbol_info:
                raise ValueError(f"Symbol {symbol} not found")

            filling_mode = symbol_info.filling_mode

            if filling_mode & 1:
                return mt5.ORDER_FILLING_FOK
            elif filling_mode & 2:
                return mt5.ORDER_FILLING_IOC
            elif filling_mode & 4:
                return mt5.ORDER_FILLING_RETURN
            else:
                raise ValueError(f"No supported filling modes for {symbol}")
        except Exception as e:
            raise

    def close_order(self, ticket, symbol, volume, order_type):
        try:
            request = {
                "action": mt5.TRADE_ACTION_CLOSE,
                "position": ticket if order_type in [mt5.POSITION_TYPE_BUY, mt5.POSITION_TYPE_SELL] else None,
                "order": ticket if order_type in [mt5.ORDER_TYPE_BUY_LIMIT, mt5.ORDER_TYPE_SELL_LIMIT,
                                                  mt5.ORDER_TYPE_BUY_STOP, mt5.ORDER_TYPE_SELL_STOP] else None,
                "symbol": symbol,
                "volume": volume,
                "type": mt5.ORDER_TYPE_BUY if order_type == mt5.POSITION_TYPE_SELL else mt5.ORDER_TYPE_SELL,
                "type_time": mt5.ORDER_TIME_GTC,
                "type_filling": self.get_symbol_filling_mode(symbol),
            }
            result = mt5.order_send(request)
            if result.retcode == mt5.TRADE_RETCODE_DONE:
                return True
            else:
                return False
        except Exception as e:
            return False

    def order_send(self, request):
        try:
            result = mt5.order_send(request)
            if result is None:
                logger.error(f"Failed to send order: {request}")
            return result
        except Exception as e:
            return None
        
    def shutdown(self):
        try:
            with self.margin_lock:
                if mt5.terminal_info() is not None:
                    mt5.shutdown()
                    logger.info("MetaTrader 5 connection successfully closed")
                else:
                    logger.warning("No active MetaTrader 5 connection to close")
        except Exception as e:
            logger.error(f"Error during shutdown: {str(e)}")

    def execute_market_trade(self, trade: PoolTrade, price: float) -> bool:
        try:
            order_type = mt5.ORDER_TYPE_BUY if trade.trade_type == "BUY" else mt5.ORDER_TYPE_SELL
            filling_mode = self.get_symbol_filling_mode(trade.symbol)

            request = {
                "action": mt5.TRADE_ACTION_DEAL,
                "symbol": trade.symbol,
                "volume": trade.volume,
                "type": order_type,
                "price": price,
                "deviation": trade.slippage,
                "magic": trade.magic_number,
                "comment": trade.comment,
                "type_time": mt5.ORDER_TIME_GTC,
                "type_filling": filling_mode,
            }

            result = mt5.order_send(request)
            if result.retcode == mt5.TRADE_RETCODE_DONE:
                logger.info(f"Market trade executed: {result}")
                return True
            else:
                logger.error(f"Market trade failed: {result}")
                return False
        except Exception as e:
            logger.error(f"Exception in execute_market_trade: {e}")
            return False

    def place_pending_order(self, trade: PoolTrade) -> bool:
        try:
            if trade.trade_type == "BUY_LIMIT":
                order_type = mt5.ORDER_TYPE_BUY_LIMIT
            elif trade.trade_type == "SELL_LIMIT":
                order_type = mt5.ORDER_TYPE_SELL_LIMIT
            elif trade.trade_type == "BUY_STOP":
                order_type = mt5.ORDER_TYPE_BUY_STOP
            elif trade.trade_type == "SELL_STOP":
                order_type = mt5.ORDER_TYPE_SELL_STOP
            else:
                logger.error(f"Unsupported pending trade type: {trade.trade_type}")
                return False

            filling_mode = self.get_symbol_filling_mode(trade.symbol)

            request = {
                "action": mt5.TRADE_ACTION_PENDING,
                "symbol": trade.symbol,
                "volume": trade.volume,
                "type": order_type,
                "price": trade.price,
                "sl": trade.sl,
                "tp": trade.tp,
                "deviation": trade.slippage,
                "magic": trade.magic_number,
                "comment": trade.comment,
                "type_time": mt5.ORDER_TIME_GTC,
                "type_filling": filling_mode,
            }

            result = mt5.order_send(request)
            if result.retcode == mt5.TRADE_RETCODE_DONE:
                logger.info(f"Pending order placed: {result}")
                return True
            else:
                logger.error(f"Pending order failed: {result}")
                return False
        except Exception as e:
            logger.error(f"Exception in place_pending_order: {e}")
            return False