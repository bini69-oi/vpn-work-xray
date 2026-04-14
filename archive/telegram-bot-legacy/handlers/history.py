from __future__ import annotations

import httpx
from telegram import Update
from telegram.ext import ContextTypes

from handlers.common import (
    api_client,
    api_error_message,
    deny_if_not_allowed,
    reply_text,
    handle_request_error,
)
from keyboards import back_to_menu, main_menu
from texts import NO_SUBSCRIPTION
from utils import user_id_for_api


async def cmd_history(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    uid = update.effective_user.id
    client = api_client(context)
    if client is None:
        cfg = context.bot_data.get("cfg")
        mini = cfg.miniapp_url if cfg else ""
        await reply_text(update, context, "Режим проверки без сервера (BOT_DRY_RUN=1).", reply_markup=main_menu(False, mini))
        return
    try:
        status, data = await client.issue_history(user_id_for_api(uid), limit=10)
    except httpx.RequestError as e:
        await handle_request_error(update, context, e)
        return
    if status == 404:
        await reply_text(update, context, NO_SUBSCRIPTION, reply_markup=back_to_menu())
        return
    if status != 200:
        await reply_text(update, context, f"Ошибка API ({status}): {api_error_message(data)}", reply_markup=back_to_menu())
        return
    items = data.get("items") or []
    if not isinstance(items, list) or not items:
        await reply_text(update, context, "История выдач пуста.", reply_markup=back_to_menu())
        return
    out: list[str] = ["Последние выдачи:"]
    for i, it in enumerate(items[:10], 1):
        if not isinstance(it, dict):
            continue
        sid = it.get("subscriptionId", "—")
        hint = it.get("tokenHint") or "—"
        issued = it.get("issuedAt", "")
        exp = it.get("expiresAt", "")
        out.append(f"{i}. sub {sid}, hint {hint}")
        out.append(f"   выдано: {issued}  до: {exp}")
    await reply_text(update, context, "\n".join(out), reply_markup=back_to_menu())

