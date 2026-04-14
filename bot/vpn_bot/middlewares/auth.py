from __future__ import annotations

import logging
from typing import Any, Awaitable, Callable

import aiosqlite
from aiogram import BaseMiddleware
from aiogram.types import CallbackQuery, Message, TelegramObject

from vpn_bot.config import settings
from vpn_bot.database.migrations import REPLY_KEYBOARD_VERSION
from vpn_bot.services.subscription_service import resolve_reply_main_menu, vpn_user_id
from vpn_bot.utils import texts

log = logging.getLogger(__name__)


def _event_user(event: TelegramObject):
    if isinstance(event, Message):
        return event.from_user
    if isinstance(event, CallbackQuery):
        return event.from_user
    return getattr(event, "from_user", None)


class AuthMiddleware(BaseMiddleware):
    async def __call__(
        self,
        handler: Callable[[TelegramObject, dict[str, Any]], Awaitable[Any]],
        event: TelegramObject,
        data: dict[str, Any],
    ) -> Any:
        user = _event_user(event)
        if user is None:
            return await handler(event, data)

        uid = user.id
        allowed = settings.allowed_ids()
        if allowed is not None and uid not in allowed:
            if isinstance(event, Message):
                await event.answer(texts.access_denied())
            elif isinstance(event, CallbackQuery):
                await event.answer()
                if event.message:
                    await event.message.answer(texts.access_denied())
            return None

        path = settings.database_file()
        path.parent.mkdir(parents=True, exist_ok=True)
        async with aiosqlite.connect(path) as db:
            db.row_factory = aiosqlite.Row
            await db.execute(
                """
                INSERT INTO users (telegram_id, username, full_name, vpn_user_id)
                VALUES (?, ?, ?, ?)
                ON CONFLICT(telegram_id) DO UPDATE SET
                    username = excluded.username,
                    full_name = excluded.full_name,
                    vpn_user_id = excluded.vpn_user_id,
                    updated_at = CURRENT_TIMESTAMP
                """,
                (
                    uid,
                    user.username or "",
                    user.full_name or "",
                    vpn_user_id(uid),
                ),
            )
            await db.commit()
            cur = await db.execute("SELECT * FROM users WHERE telegram_id = ?", (uid,))
            row = await cur.fetchone()
            if row and int(row["is_banned"] or 0):
                if isinstance(event, Message):
                    await event.answer(texts.banned())
                elif isinstance(event, CallbackQuery):
                    await event.answer()
                    if event.message:
                        await event.message.answer(texts.banned())
                return None

            disp = data.get("dispatcher")
            api = None
            if disp is not None:
                try:
                    wf = getattr(disp, "workflow_data", None)
                    if isinstance(wf, dict):
                        api = wf.get("api_client")
                except Exception:
                    api = None
            if api is None and settings.vpn_api_token.strip():
                log.error("api_client missing on dispatcher (VPN_API_TOKEN is set)")

            try:
                mv_cur = int(row["reply_menu_version"] or 0) if row else 0
            except (KeyError, TypeError, ValueError):
                mv_cur = 0
            if mv_cur < REPLY_KEYBOARD_VERSION:
                note = texts.menu_updated_notice()
                kb = await resolve_reply_main_menu(api, uid)
                if isinstance(event, Message):
                    await event.answer(note, reply_markup=kb)
                elif isinstance(event, CallbackQuery) and event.from_user:
                    await event.bot.send_message(event.from_user.id, note, reply_markup=kb)
                await db.execute(
                    "UPDATE users SET reply_menu_version = ?, updated_at = CURRENT_TIMESTAMP WHERE telegram_id = ?",
                    (REPLY_KEYBOARD_VERSION, uid),
                )
                await db.commit()

            data["db"] = db
            data["api"] = api
            return await handler(event, data)
