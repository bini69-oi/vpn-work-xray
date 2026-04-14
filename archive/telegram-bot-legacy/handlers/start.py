from __future__ import annotations

from telegram import Update
from telegram.ext import ContextTypes

from handlers.common import deny_if_not_allowed, is_admin
from keyboards import main_menu
from texts import HELP_TEXT, START_TEXT


async def cmd_start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    uid = update.effective_user.id if update.effective_user else 0
    cfg = context.bot_data["cfg"]
    await update.message.reply_text(START_TEXT, reply_markup=main_menu(is_admin(context, uid), cfg.miniapp_url))


async def cmd_help(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    uid = update.effective_user.id if update.effective_user else 0
    cfg = context.bot_data["cfg"]
    await update.message.reply_text(HELP_TEXT, reply_markup=main_menu(is_admin(context, uid), cfg.miniapp_url))

