import asyncio
import websockets
import json
from config.settings import settings
from services.trade_manager import TradeManager
from utils.logger import logger
from websockets.exceptions import ConnectionClosed

class WebSocketClient:
    def __init__(self, trade_manager: TradeManager):
        self.trade_manager = trade_manager
        self.websocket = None
        self.last_ping_sent = 0
        self.reconnect_attempts = 0
        self.missed_pongs = 0

    async def initialize(self):
        await self.connect()

    async def connect(self):
        full_url = f"ws://{settings.WEBSOCKET_URL}:{settings.WEBSOCKET_PORT}{settings.WEBSOCKET_PATH}"
        logger.info(f"Attempting to connect to WebSocket server: {full_url}")
        backoff = settings.RECONNECT_BACKOFF_INITIAL
        while self.reconnect_attempts < settings.MAX_RECONNECT_ATTEMPTS:
            try:
                self.websocket = await websockets.connect(full_url, ping_interval=None, max_size=settings.MAX_MESSAGE_SIZE)
                if await self.send_handshake():
                    logger.info(f"Connected to WebSocket server: {full_url}")
                    self.reconnect_attempts = 0
                    self.missed_pongs = 0
                    return True
            except Exception as e:
                logger.error(f"Failed to connect to WebSocket server: {str(e)}")
                self.reconnect_attempts += 1
                await asyncio.sleep(backoff)
                backoff = min(backoff * 1.5, settings.RECONNECT_BACKOFF_MAX)
        logger.error(f"Max reconnection attempts reached ({settings.MAX_RECONNECT_ATTEMPTS})")
        return False

    async def send_handshake(self):
        handshake = {
            "type": "handshake",
            "client_id": settings.CLIENT_ID,
            "timestamp": float(self.trade_manager.mt5_client.get_symbol_tick(settings.SYMBOL).time)
        }
        try:
            await self.websocket.send(json.dumps(handshake))
            logger.info(f"Sent handshake: {json.dumps(handshake)}")
            return True
        except Exception as e:
            logger.error(f"Failed to send handshake: {str(e)}")
            return False

    async def process_messages(self):
        while True:
            try:
                message = await asyncio.wait_for(self.websocket.recv(), timeout=settings.READ_TIMEOUT)
                logger.info(f"Received: {message}")
                try:
                    json_data = json.loads(message)
                    msg_type = json_data.get("type", "")
                    if msg_type == "handshake_response":
                        self.reconnect_attempts = 0
                        logger.info("Received handshake response")
                    elif msg_type == "trade_request":
                        await self.trade_manager.handle_trade_request(json_data, self.websocket)
                    elif msg_type == "balance_request":
                        await self.trade_manager.handle_balance_request(json_data, self.websocket)
                    elif msg_type == "close_trade_request":
                        await self.trade_manager.handle_close_trade_request(json_data, self.websocket)
                    elif msg_type == "order_stream_request":
                        await self.trade_manager.handle_order_stream_request(json_data, self.websocket)
                    elif msg_type == "ping":
                        pong = {"type": "pong", "timestamp": float(self.trade_manager.mt5_client.get_symbol_tick(settings.SYMBOL).time)}
                        await self.websocket.send(json.dumps(pong))
                        logger.info(f"Sent pong: {json.dumps(pong)}")
                        self.missed_pongs = 0
                    else:
                        logger.warning(f"Unknown message type: {msg_type}")
                except json.JSONDecodeError:
                    logger.error(f"Invalid JSON: {message}")
            except asyncio.TimeoutError:
                logger.warning("Read timeout, checking connection")
                self.missed_pongs += 1
                if self.missed_pongs >= 5:
                    logger.error("Too many missed pongs, reconnecting")
                    await self.reconnect()
            except ConnectionClosed:
                logger.error("WebSocket connection closed, reconnecting")
                await self.reconnect()
            except Exception as e:
                logger.error(f"WebSocket error: {str(e)}")
                await self.reconnect()

    async def handle_ping(self):
        current_time = float(self.trade_manager.mt5_client.get_symbol_tick(settings.SYMBOL).time)
        if current_time - self.last_ping_sent >= settings.PING_INTERVAL:
            ping = {"type": "ping", "timestamp": current_time}
            try:
                await self.websocket.send(json.dumps(ping))
                logger.info(f"Sent ping: {json.dumps(ping)}")
                self.last_ping_sent = current_time
            except ConnectionClosed:
                logger.error("WebSocket closed while sending ping, reconnecting")
                await self.reconnect()
            except Exception as e:
                logger.error(f"Failed to send ping: {str(e)}")
                self.trade_manager.trade_repository.queue_message(json.dumps(ping))

    async def reconnect(self):
        if self.websocket:
            try:
                await self.websocket.close()
            except:
                pass
        self.websocket = None
        if await self.connect():
            for message in self.trade_manager.trade_repository.message_queue[:]:
                try:
                    await self.websocket.send(message)
                    logger.info(f"Resent queued message: {message}")
                    self.trade_manager.trade_repository.message_queue.remove(message)
                except Exception as e:
                    logger.error(f"Failed to resend queued message: {str(e)}")

    async def deinitialize(self):
        if self.websocket:
            disconnect = {
                "type": "disconnect",
                "reason": "Client shutdown",
                "timestamp": float(self.trade_manager.mt5_client.get_symbol_tick(settings.SYMBOL).time)
            }
            try:
                await self.websocket.send(json.dumps(disconnect))
                await self.websocket.close()
            except Exception as e:
                logger.error(f"Failed to send disconnect message: {str(e)}")
        self.websocket = None