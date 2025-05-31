import MetaTrader5 as mt5
from threading import Lock
from utils.logger import logger

class MT5Client:
    def __init__(self):
        self.margin_lock = Lock()
        if not mt5.initialize():
            logger.error("MT5 initialization failed")
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
                    logger.error(f"Failed to get symbol info for {symbol}")
                    return False

                tick = self.get_symbol_tick(symbol)
                if not tick:
                    logger.error(f"Failed to get tick data for {symbol}")
                    return False
                market_price = tick.bid if trade_type == "SELL" else tick.ask

                account_info = self.get_account_info()
                if not account_info:
                    logger.error("Failed to get account info")
                    return False

                contract_size = symbol_info.trade_contract_size
                leverage = account_info.leverage
                required_margin = (volume * contract_size * market_price) / leverage

                free_margin = account_info.margin_free

                logger.info(
                    f"Margin check for {symbol}: "
                    f"Volume={volume}, ContractSize={contract_size}, Price={market_price}, "
                    f"Leverage={leverage}, RequiredMargin={required_margin}, FreeMargin={free_margin}"
                )

                return free_margin >= required_margin
            except Exception as e:
                logger.error(f"Error checking margin for {symbol}: {str(e)}")
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
                logger.error(f"Cannot get filling mode: Symbol {symbol} not found")
                raise ValueError(f"Symbol {symbol} not found")

            filling_mode = symbol_info.filling_mode

            if filling_mode & 1:
                return mt5.ORDER_FILLING_FOK
            elif filling_mode & 2:
                return mt5.ORDER_FILLING_IOC
            elif filling_mode & 4:
                return mt5.ORDER_FILLING_RETURN
            else:
                logger.error(f"No supported filling modes for {symbol}")
                raise ValueError(f"No supported filling modes for {symbol}")
        except Exception as e:
            logger.error(f"Error getting filling mode for {symbol}: {str(e)}")
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
                logger.info(f"Closed order/position {ticket} for {symbol}")
                return True
            else:
                logger.error(f"Failed to close order/position {ticket} for {symbol}: {result.comment}")
                return False
        except Exception as e:
            logger.error(f"Error closing order/position {ticket} for {symbol}: {str(e)}")
            return False

    def order_send(self, request):
        try:
            result = mt5.order_send(request)
            if result is None:
                logger.error(f"Failed to send order: {request}")
            return result
        except Exception as e:
            logger.error(f"Error sending order: {str(e)}")
            return None