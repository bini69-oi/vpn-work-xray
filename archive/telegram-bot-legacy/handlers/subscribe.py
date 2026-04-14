from __future__ import annotations

import httpx
from telegram import Update
from telegram.ext import ContextTypes

from handlers.common import (
    api_client,
    api_error_message,
    check_subscribe_rate_limit,
    deny_if_not_allowed,
    handle_request_error,
    reply_text,
)
from keyboards import back_to_menu, subscribe_choice
from utils import user_id_for_api


async def cmd_subscribe(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    uid = update.effective_user.id

    ok, wait = check_subscribe_rate_limit(context, uid)
    if not ok:
        await reply_text(update, context, f"Слишком часто. Попробуйте через {wait} сек.", reply_markup=back_to_menu())
        return

    client = api_client(context)
    if client is None:
        await reply_text(update, context, "Режим проверки без сервера (BOT_DRY_RUN=1).", reply_markup=back_to_menu())
        return

    api_uid = user_id_for_api(uid)

    # If subscription exists, ask what to do.
    try:
        st, _ = await client.issue_status(api_uid)
    except httpx.RequestError as e:
        await handle_request_error(update, context, e)
        return
    if st == 200:
        await reply_text(update, context, "У вас уже есть подписка. Продлить или создать новую?", reply_markup=subscribe_choice())
        return

    profile_ids = context.bot_data.get("profile_ids")
    if not isinstance(profile_ids, list):
        profile_ids = None
    name = update.effective_user.full_name if update.effective_user else "telegram-user"
    idem = str(update.update_id)
    try:
        status, data = await client.issue_link(
            user_id=api_uid,
            name=name,
            source="telegram",
            profile_ids=profile_ids,
            idempotency_key=idem,
        )
    except httpx.RequestError as e:
        await handle_request_error(update, context, e)
        return
    if status != 200:
        await reply_text(update, context, f"Ошибка API ({status}): {api_error_message(data)}", reply_markup=back_to_menu())
        return
    url = str(data.get("url") or "").strip()
    if url:
        await reply_text(update, context, f"Подписка выдана.\n\nСсылка:\n{url}", reply_markup=back_to_menu())
        return
    await reply_text(update, context, "Подписка выдана, но ссылка не пришла (проверьте public base URL на сервере).", reply_markup=back_to_menu())

