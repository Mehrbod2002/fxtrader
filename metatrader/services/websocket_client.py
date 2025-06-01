import asyncio
import websockets
import json
import time
import redis
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
        self.redis_client = redis.Redis(
            host=settings.REDIS_HOST, port=settings.REDIS_PORT, db=0)

    async def initialize(self):
        return await self.connect()

    async def connect(self):
        full_url = f"ws://{settings.WEBSOCKET_URL}:{settings.WEBSOCKET_PORT}{settings.WEBSOCKET_PATH}"
        backoff = settings.RECONNECT_BACKOFF_INITIAL
        while self.reconnect_attempts < settings.MAX_RECONNECT_ATTEMPTS:
            try:
                self.websocket = await websockets.connect(full_url, ping_interval=None, max_size=settings.MAX_MESSAGE_SIZE)
                if await self.send_handshake():
                    self.reconnect_attempts = 0
                    self.missed_pongs = 0
                    return True
            except Exception as e:
                logger.error(
                    f"Failed to connect to WebSocket server: {str(e)}")
                self.reconnect_attempts += 1
                await asyncio.sleep(backoff)
                backoff = min(backoff * 1.5, settings.RECONNECT_BACKOFF_MAX)

        logger.error(
            f"Max reconnection attempts reached ({settings.MAX_RECONNECT_ATTEMPTS})")
        return False

    async def send_handshake(self):
        handshake = {
            "type": "handshake",
            "client_id": settings.CLIENT_ID,
            "timestamp": float(self.trade_manager.get_timestamp())
        }
        try:
            await self.websocket.send(json.dumps(handshake))
            return True
        except Exception as e:
            return False

    async def send_ping(self):
        while True:
            try:
                ping = {"type": "ping", "timestamp": float(
                    self.trade_manager.get_timestamp())}
                await self.websocket.send(json.dumps(ping))
                await asyncio.sleep(settings.PING_INTERVAL)
            except Exception as e:
                logger.error(f"Error sending ping: {str(e)}")
                await self.reconnect()

    async def process_messages(self):
        while True:
            try:
                message = await asyncio.wait_for(self.websocket.recv(), timeout=settings.READ_TIMEOUT)

                try:
                    json_data = json.loads(message)
                    msg_type = json_data.get("type", "")
                    if msg_type == "handshake_response":
                        self.reconnect_attempts = 0
                    elif msg_type == "trade_request":
                        await self.trade_manager.handle_trade_request(json_data, self.websocket)
                    elif msg_type == "balance_request":
                        await self.trade_manager.handle_balance_request(json_data, self.websocket)
                    elif msg_type == "close_trade_request":
                        await self.trade_manager.handle_close_trade_request(json_data, self.websocket)
                    elif msg_type == "order_stream_request":
                        user_id = json_data.get("user_id", "")
                        account_type = json_data.get("account_type", "")
                        asyncio.create_task(self.trade_manager.stream_orders(
                            user_id, account_type, self.websocket))
                    elif msg_type == "modify_trade_request":
                        await self.trade_manager.handle_modify_trade_request(json_data, self.websocket)
                    elif msg_type == "ping":
                        pong = {"type": "pong", "timestamp": float(
                            self.trade_manager.get_timestamp())}
                        await self.websocket.send(json.dumps(pong))
                        self.missed_pongs = 0
                    elif msg_type == "pong":
                        self.missed_pongs = 0
                    else:
                        logger.warning(f"Unknown message type: {msg_type}")
                except json.JSONDecodeError:
                    logger.error(f"Invalid JSON: {message}")
                except ConnectionClosed:
                    await self.reconnect()
                except asyncio.TimeoutError:
                    self.missed_pongs += 1
                    if self.missed_pongs >= 5:
                        await self.reconnect()
            except asyncio.TimeoutError:
                self.missed_pongs += 1
                if self.missed_pongs >= 5:
                    await self.reconnect()
            except ConnectionClosed:
                await self.reconnect()

    async def handle_ping(self):
        current_time = float(self.trade_manager.get_timestamp())
        if current_time - self.last_ping_sent >= settings.PING_INTERVAL:
            ping = {"type": "ping", "timestamp": current_time}
            try:
                await self.websocket.send(json.dumps(ping))
                self.last_ping_sent = current_time
            except ConnectionClosed:
                await self.reconnect()
            except Exception as e:
                self.trade_manager.trade_repository.queue_message(
                    json.dumps(ping))

    async def reconnect(self):
        if self.websocket:
            try:
                await self.websocket.close()
            except:
                pass
        self.websocket = None
        self.reconnect_attempts += 1
        if self.reconnect_attempts >= settings.MAX_RECONNECT_ATTEMPTS:
            logger.error("Max reconnect attempts reached")
            return
        if await self.connect():
            while self.redis_client.llen("message_queue") > 0:
                message = self.redis_client.lpop("message_queue").decode()
                try:
                    await self.websocket.send(message)
                except Exception as e:
                    self.redis_client.lpush("message_queue", message)

    async def deinitialize(self):
        if self.websocket:
            disconnect = {
                "type": "disconnect",
                "reason": "Client shutdown",
                "timestamp": float(self.trade_manager.get_timestamp() or time.time())
            }
            try:
                await self.websocket.send(json.dumps(disconnect))
                await self.websocket.close()
            except Exception as e:
                ...
        self.websocket = None
