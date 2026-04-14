from __future__ import annotations

import httpx
from telegram import Update
from telegram.ext import ContextTypes

from handlers.common import api_client, api_error_message, deny_if_not_allowed, handle_request_error, reply_text
from keyboards import back_to_menu
from utils import user_profile_id


async def cmd_links(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    uid = update.effective_user.id
    client = api_client(context)
    if client is None:
        await reply_text(update, context, "Режим проверки без сервера (BOT_DRY_RUN=1).", reply_markup=back_to_menu())
        return
    pid = user_profile_id(uid)
    try:
        st, data = await client.get_delivery_links(pid)
    except httpx.RequestError as e:
        await handle_request_error(update, context, e)
        return
    if st != 200:
        await reply_text(update, context, f"Ошибка API ({st}): {api_error_message(data)}", reply_markup=back_to_menu())
        return

    links = data.get("links")
    if isinstance(links, dict):
        items = links
    else:
        items = {}

    lines = [
        f"🌐 Профиль: {pid}",
        "",
        "Ссылки подключения:",
    ]
    for k, v in items.items():
        if isinstance(v, str) and v.strip():
            lines.append(f"- {k}: {v.strip()}")
    lines.extend(
        [
            "",
            "Рекомендации по клиентам:",
            "iOS: Streisand или V2Box",
            "Android: v2rayNG",
            "Windows: Hiddify",
            "macOS: V2Box",
        ]
    )

    await reply_text(update, context, "\n".join(lines), reply_markup=back_to_menu())

