from __future__ import annotations

import httpx
from telegram import Update
from telegram.ext import ContextTypes

from handlers.common import api_client, api_error_message, deny_if_not_allowed, handle_request_error, reply_text
from keyboards import back_to_menu
from texts import NO_SUBSCRIPTION
from utils import format_date, user_id_for_api


async def cmd_renew(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    uid = update.effective_user.id
    client = api_client(context)
    if client is None:
        await reply_text(update, context, "Режим проверки без сервера (BOT_DRY_RUN=1). Продление не выполнялось.", reply_markup=back_to_menu())
        return
    try:
        st, data = await client.issue_status(user_id_for_api(uid))
    except httpx.RequestError as e:
        await handle_request_error(update, context, e)
        return
    if st == 404:
        await reply_text(update, context, NO_SUBSCRIPTION, reply_markup=back_to_menu())
        return
    if st != 200:
        await reply_text(update, context, f"Ошибка API ({st}): {api_error_message(data)}", reply_markup=back_to_menu())
        return
    try:
        st2, data2 = await client.lifecycle(user_id_for_api(uid), action="renew", days=30)
    except httpx.RequestError as e:
        await handle_request_error(update, context, e)
        return
    if st2 != 200:
        await reply_text(update, context, f"Ошибка API ({st2}): {api_error_message(data2)}", reply_markup=back_to_menu())
        return
    await reply_text(update, context, f"✅ Подписка продлена до {format_date(data2.get('expiresAt'))}", reply_markup=back_to_menu())

