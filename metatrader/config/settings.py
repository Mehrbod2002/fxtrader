class Settings:
    MT5_LOGIN = 90437505
    MT5_PASSWORD = "AMir87808780@@"
    MT5_SERVER = "LiteFinance-MT5-Demo"
    MT5_PATH = "C:\\Program Files\\MetaTrader 5\\terminal64.exe"
    SYMBOL = "EURUSD_o"
    WEBSOCKET_URL = "127.0.0.1"
    WEBSOCKET_PORT = 7003  # Updated to match Go server
    WEBSOCKET_PATH = "/ws"
    CLIENT_ID = "MT5_Client_1"
    PING_INTERVAL = 30  # seconds
    RECONNECT_BACKOFF_INITIAL = 2  # seconds
    RECONNECT_BACKOFF_MAX = 30  # seconds
    MAX_RECONNECT_ATTEMPTS = 5
    SPREAD_TOLERANCE = 0.0001
    TIMER_INTERVAL = 0.5  # seconds
    READ_TIMEOUT = 120  # seconds
    WRITE_TIMEOUT = 10  # seconds
    MAX_MESSAGE_SIZE = 1024 * 1024  # 1MB

settings = Settings()