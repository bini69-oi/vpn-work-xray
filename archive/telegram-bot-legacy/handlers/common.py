from __future__ import annotations

import logging
import time
from typing import Any

import httpx
from telegram import Update
from telegram.ext import ContextTypes

from api_client import VPNProductClient
from config import BotConfig
from texts import ACCESS_DENIED, API_UNAVAILABLE
from utils import check_rate_limit

logger = logging.getLogger(__name__)


def cfg(context: ContextTypes.DEFAULT_TYPE) -> BotConfig:
    return context.bot_data["cfg"]


def is_admin(context: ContextTypes.DEFAULT_TYPE, telegram_user_id: int) -> bool:
    return telegram_user_id in cfg(context).admin_telegram_ids


def is_allowed(context: ContextTypes.DEFAULT_TYPE, telegram_user_id: int) -> bool:
    allowed = cfg(context).allowed_telegram_ids
    if allowed is None:
        return True
    return telegram_user_id in allowed


async def deny_if_not_allowed(update: Update, context: ContextTypes.DEFAULT_TYPE) -> bool:
    uid = update.effective_user.id if update.effective_user else 0
    if is_allowed(context, uid):
        return False
    if update.callback_query:
        await update.callback_query.answer(ACCESS_DENIED, show_alert=True)
        return True
    if update.message:
        await update.message.reply_text(ACCESS_DENIED)
        return True
    return True


def api_client(context: ContextTypes.DEFAULT_TYPE) -> VPNProductClient | None:
    return context.bot_data.get("vpn_client")


def api_error_message(data: dict[str, Any]) -> str:
    err = data.get("error") or data.get("message") or "неизвестная ошибка"
    code = data.get("code")
    if code:
        return f"{err} ({code})"
    return str(err)


def check_subscribe_rate_limit(context: ContextTypes.DEFAULT_TYPE, telegram_user_id: int) -> tuple[bool, int]:
    key = f"rl_subscribe:{telegram_user_id}"
    now = time.time()
    last_ts = context.bot_data.get(key)
    res = check_rate_limit(last_ts if isinstance(last_ts, (int, float)) else None, now, cfg(context).rate_limit_seconds)
    if res.ok:
        context.bot_data[key] = now
        return True, 0
    return False, res.wait_seconds


async def reply_text(update: Update, context: ContextTypes.DEFAULT_TYPE, text: str, reply_markup=None) -> None:
    if update.callback_query:
        await update.callback_query.edit_message_text(text, reply_markup=reply_markup)
        return
    if update.message:
        await update.message.reply_text(text, reply_markup=reply_markup)


async def handle_request_error(update: Update, context: ContextTypes.DEFAULT_TYPE, exc: httpx.RequestError) -> None:
    logger.warning("api request failed: %s", exc)
    await reply_text(update, context, API_UNAVAILABLE)

