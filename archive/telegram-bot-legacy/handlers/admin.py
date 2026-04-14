from __future__ import annotations

import httpx
from telegram import Update
from telegram.ext import ContextTypes

from handlers.common import api_client, api_error_message, deny_if_not_allowed, handle_request_error, is_admin, reply_text
from keyboards import back_to_menu
from utils import user_id_for_api


async def cmd_admin(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    uid = update.effective_user.id
    if not is_admin(context, uid):
        await reply_text(update, context, "Недостаточно прав.", reply_markup=back_to_menu())
        return
    await reply_text(
        update,
        context,
        "Админ-команды:\n"
        "/admin_stats\n"
        "/admin_health\n"
        "/admin_block tg_123456\n"
        "/admin_unblock tg_123456\n",
        reply_markup=back_to_menu(),
    )


async def cmd_admin_health(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    uid = update.effective_user.id
    if not is_admin(context, uid):
        await reply_text(update, context, "Недостаточно прав.", reply_markup=back_to_menu())
        return
    client = api_client(context)
    if client is None:
        await reply_text(update, context, "dry-run", reply_markup=back_to_menu())
        return
    try:
        st, data = await client.get_health()
    except httpx.RequestError as e:
        await handle_request_error(update, context, e)
        return
    if st not in (200, 503):
        await reply_text(update, context, f"Ошибка API ({st}): {api_error_message(data)}", reply_markup=back_to_menu())
        return
    await reply_text(update, context, f"Health ({st}):\n{data}", reply_markup=back_to_menu())


async def cmd_admin_stats(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    uid = update.effective_user.id
    if not is_admin(context, uid):
        await reply_text(update, context, "Недостаточно прав.", reply_markup=back_to_menu())
        return
    client = api_client(context)
    if client is None:
        await reply_text(update, context, "dry-run", reply_markup=back_to_menu())
        return
    try:
        st, data = await client.get_profile_stats()
    except httpx.RequestError as e:
        await handle_request_error(update, context, e)
        return
    if st != 200:
        await reply_text(update, context, f"Ошибка API ({st}): {api_error_message(data)}", reply_markup=back_to_menu())
        return
    items = data.get("items") or []
    count = len(items) if isinstance(items, list) else 0
    await reply_text(update, context, f"Профилей: {count}", reply_markup=back_to_menu())


async def cmd_admin_block(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    uid = update.effective_user.id
    if not is_admin(context, uid):
        await reply_text(update, context, "Недостаточно прав.", reply_markup=back_to_menu())
        return
    if not context.args:
        await reply_text(update, context, "Использование: /admin_block tg_123456", reply_markup=back_to_menu())
        return
    target = str(context.args[0]).strip()
    client = api_client(context)
    if client is None:
        await reply_text(update, context, "dry-run", reply_markup=back_to_menu())
        return
    try:
        st, data = await client.lifecycle(target, action="block")
    except httpx.RequestError as e:
        await handle_request_error(update, context, e)
        return
    if st != 200:
        await reply_text(update, context, f"Ошибка API ({st}): {api_error_message(data)}", reply_markup=back_to_menu())
        return
    await reply_text(update, context, "OK: blocked", reply_markup=back_to_menu())


async def cmd_admin_unblock(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    uid = update.effective_user.id
    if not is_admin(context, uid):
        await reply_text(update, context, "Недостаточно прав.", reply_markup=back_to_menu())
        return
    if not context.args:
        await reply_text(update, context, "Использование: /admin_unblock tg_123456", reply_markup=back_to_menu())
        return
    target = str(context.args[0]).strip()
    client = api_client(context)
    if client is None:
        await reply_text(update, context, "dry-run", reply_markup=back_to_menu())
        return
    try:
        st, data = await client.lifecycle(target, action="renew", days=30)
    except httpx.RequestError as e:
        await handle_request_error(update, context, e)
        return
    if st != 200:
        await reply_text(update, context, f"Ошибка API ({st}): {api_error_message(data)}", reply_markup=back_to_menu())
        return
    await reply_text(update, context, "OK: renewed", reply_markup=back_to_menu())

