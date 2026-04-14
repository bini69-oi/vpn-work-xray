from __future__ import annotations

import asyncio
import logging
import os

import aiohttp
from aiogram import Bot, Dispatcher
from aiogram.client.default import DefaultBotProperties
from aiogram.client.session.aiohttp import AiohttpSession
from aiogram.enums import ParseMode
from aiogram.exceptions import TelegramNetworkError
from aiogram.fsm.storage.memory import MemoryStorage

from vpn_bot.config import settings
from vpn_bot.database.db import init_db
from vpn_bot.handlers import admin, purchase, referral, start, subscription, support
from vpn_bot.middlewares.auth import AuthMiddleware
from vpn_bot.middlewares.throttling import ThrottlingMiddleware
from vpn_bot.services.api_client import VPNApiClient

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(name)s] %(levelname)s: %(message)s",
)
log = logging.getLogger(__name__)

_GET_ME_ATTEMPTS = 5
_GET_ME_DELAYS_SEC = (1.5, 3.0, 5.0, 8.0)


async def _bot_get_me(bot: Bot):
    last: TelegramNetworkError | None = None
    for attempt in range(_GET_ME_ATTEMPTS):
        try:
            return await bot.get_me()
        except TelegramNetworkError as e:
            last = e
            log.warning(
                "Не удалось связаться с api.telegram.org (попытка %s/%s): %s",
                attempt + 1,
                _GET_ME_ATTEMPTS,
                e,
            )
            if attempt < _GET_ME_ATTEMPTS - 1:
                await asyncio.sleep(_GET_ME_DELAYS_SEC[attempt])
    assert last is not None
    raise last


async def main() -> None:
    if not settings.bot_token.strip():
        raise SystemExit("BOT_TOKEN is required (see apps/vpn-telegram-bot/.env.example)")

    await init_db()

    proxy = (settings.telegram_proxy_url or "").strip()
    if proxy:
        os.environ.setdefault("HTTPS_PROXY", proxy)
        os.environ.setdefault("HTTP_PROXY", proxy)
        log.info("TELEGRAM_PROXY задан — запросы к Telegram идут через прокси")

    vpn_session = aiohttp.ClientSession()
    tg_session = AiohttpSession(proxy=proxy or None) if proxy else AiohttpSession()

    api: VPNApiClient | None = None
    if settings.vpn_api_token.strip():
        api = VPNApiClient(vpn_session, settings.vpn_api_url, settings.vpn_api_token)
    else:
        log.warning("VPN_API_TOKEN empty — API calls disabled")

    bot = Bot(
        token=settings.bot_token,
        session=tg_session,
        default=DefaultBotProperties(parse_mode=ParseMode.HTML),
    )
    dp = Dispatcher(storage=MemoryStorage())
    dp["api_client"] = api

    try:
        me = await _bot_get_me(bot)
    except TelegramNetworkError as e:
        log.error(
            "Нет соединения с Telegram (api.telegram.org). Проверь интернет, VPN или задай "
            "в .env приложения (apps/vpn-telegram-bot/) задай TELEGRAM_PROXY=http://127.0.0.1:ПОРТ для локального прокси. Ошибка: %s",
            e,
        )
        await tg_session.close()
        await vpn_session.close()
        raise SystemExit(1) from e

    dp["bot_username"] = me.username or ""

    dp.message.middleware(ThrottlingMiddleware())
    dp.callback_query.middleware(ThrottlingMiddleware())
    dp.message.middleware(AuthMiddleware())
    dp.callback_query.middleware(AuthMiddleware())

    dp.include_router(start.router)
    dp.include_router(admin.router)
    dp.include_router(purchase.router)
    dp.include_router(subscription.router)
    dp.include_router(referral.router)
    dp.include_router(support.router)

    log.info("bot @%s starting", me.username)
    try:
        await dp.start_polling(bot)
    finally:
        await vpn_session.close()
        await tg_session.close()
