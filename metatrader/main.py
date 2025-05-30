import asyncio
from services.mt5_client import MT5Client
from services.trade_manager import TradeManager
from services.websocket_client import WebSocketClient
from factories.trade_factory import TradeFactory
from repositories.trade_repository import TradeRepository
from config.settings import settings
from utils.logger import logger

async def main():
    mt5_client = MT5Client()
    trade_repository = TradeRepository(mt5_client)
    trade_factory = TradeFactory(mt5_client)
    trade_manager = TradeManager(mt5_client, trade_repository, trade_factory)
    ws_client = WebSocketClient(trade_manager)

    try:
        if not await ws_client.initialize():
            logger.error("Failed to initialize trade WebSocket client")
            mt5_client.shutdown()
            return

        async def background_task():
            while True:
                trade_repository.cleanup_trade_pool()
                trade_manager.process_tick()
                await ws_client.handle_ping()
                await asyncio.sleep(settings.TIMER_INTERVAL)

        await asyncio.gather(
            ws_client.process_messages(),
            ws_client.send_ping(),
            background_task()
        )
    finally:
        await ws_client.deinitialize()
        mt5_client.shutdown()

if __name__ == "__main__":
    asyncio.run(main())