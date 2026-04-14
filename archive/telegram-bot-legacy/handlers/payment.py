from __future__ import annotations

import json
import time

import httpx
from telegram import LabeledPrice, Update
from telegram.ext import ContextTypes

from config import PaymentPlan
from handlers.common import (
    api_client,
    api_error_message,
    deny_if_not_allowed,
    handle_request_error,
    is_admin,
    reply_text,
)
from keyboards import admin_payment_confirm, back_to_menu, manual_payment_confirm, pay_plans
from texts import PAYMENT_MANUAL_INSTRUCTIONS_PREFIX, PAYMENT_MANUAL_NEED_SCREENSHOT, PAYMENT_SENT_TO_ADMINS
from utils import user_id_for_api


def _plan_buttons(plans: list[PaymentPlan]) -> list:
    buttons = []
    for p in plans:
        buttons.append(("pay_plan", p))
    return buttons


async def cmd_pay(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    plans: list[PaymentPlan] = context.bot_data.get("payment_plans", [])
    btns = []
    for p in plans:
        btns.append(
            # callback_data is kept short; months is enough
            __import__("telegram").InlineKeyboardButton(f"{p.label} — {p.price_rub}₽", callback_data=f"pay_plan:{p.months}")
        )
    await reply_text(update, context, "Выберите тариф:", reply_markup=pay_plans(btns))


async def on_pay_plan(update: Update, context: ContextTypes.DEFAULT_TYPE, months: int) -> None:
    if await deny_if_not_allowed(update, context):
        return
    provider_token: str = context.bot_data.get("payment_provider_token", "")
    currency: str = context.bot_data.get("payment_currency", "RUB")
    plans: list[PaymentPlan] = context.bot_data.get("payment_plans", [])
    plan = next((p for p in plans if p.months == months), None)
    if plan is None:
        await reply_text(update, context, "Тариф не найден.", reply_markup=back_to_menu())
        return
    if provider_token:
        chat_id = update.effective_chat.id
        payload = json.dumps({"months": months, "uid": update.effective_user.id, "ts": int(time.time())})
        prices = [LabeledPrice(plan.label, int(plan.price_rub) * 100)]
        await context.bot.send_invoice(
            chat_id=chat_id,
            title="VPN подписка",
            description=f"Оплата: {plan.label}",
            payload=payload,
            provider_token=provider_token,
            currency=currency,
            prices=prices,
        )
        return

    details: str = context.bot_data.get("payment_manual_details", "")
    text = PAYMENT_MANUAL_INSTRUCTIONS_PREFIX + (details or "Реквизиты не настроены. Свяжитесь с администратором.")
    await reply_text(update, context, text, reply_markup=manual_payment_confirm())
    context.user_data["manual_payment_months"] = months
    context.user_data["awaiting_payment_screenshot"] = False


async def on_manual_paid(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    context.user_data["awaiting_payment_screenshot"] = True
    await reply_text(update, context, PAYMENT_MANUAL_NEED_SCREENSHOT, reply_markup=back_to_menu())


async def on_payment_photo(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    if not context.user_data.get("awaiting_payment_screenshot"):
        return
    months = int(context.user_data.get("manual_payment_months") or 1)
    admins: set[int] = context.bot_data.get("admin_ids", set())
    if not admins:
        await reply_text(update, context, "Нет администраторов для подтверждения оплаты.", reply_markup=back_to_menu())
        return
    caption = f"Оплата от пользователя {update.effective_user.id} на {months} мес. Подтвердить?"
    for admin_id in admins:
        try:
            if update.message and update.message.photo:
                await context.bot.send_photo(
                    chat_id=admin_id,
                    photo=update.message.photo[-1].file_id,
                    caption=caption,
                    reply_markup=admin_payment_confirm(update.effective_user.id, months),
                )
        except Exception:
            continue
    context.user_data["awaiting_payment_screenshot"] = False
    await reply_text(update, context, PAYMENT_SENT_TO_ADMINS, reply_markup=back_to_menu())


async def on_successful_payment(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    # Called by MessageHandler(filters.SUCCESSFUL_PAYMENT)
    if await deny_if_not_allowed(update, context):
        return
    uid = update.effective_user.id
    client = api_client(context)
    if client is None:
        await reply_text(update, context, "Оплата получена (dry-run).", reply_markup=back_to_menu())
        return
    payload = update.message.successful_payment.invoice_payload if update.message and update.message.successful_payment else ""
    months = 1
    try:
        p = json.loads(payload)
        months = int(p.get("months") or 1)
    except Exception:
        months = 1
    days = months * 30
    try:
        st, data = await client.lifecycle(user_id_for_api(uid), action="renew", days=days)
    except httpx.RequestError as e:
        await handle_request_error(update, context, e)
        return
    if st != 200:
        await reply_text(update, context, f"Ошибка API ({st}): {api_error_message(data)}", reply_markup=back_to_menu())
        return
    await reply_text(update, context, "✅ Оплата принята. Подписка продлена.", reply_markup=back_to_menu())


async def on_admin_confirm_payment(update: Update, context: ContextTypes.DEFAULT_TYPE, user_id: int, months: int) -> None:
    if not update.callback_query:
        return
    admin_id = update.effective_user.id if update.effective_user else 0
    if not is_admin(context, admin_id):
        await update.callback_query.answer("Недостаточно прав", show_alert=True)
        return
    client = api_client(context)
    if client is None:
        await update.callback_query.answer("dry-run", show_alert=True)
        return
    days = int(months) * 30
    try:
        st, data = await client.lifecycle(user_id_for_api(int(user_id)), action="renew", days=days)
    except httpx.RequestError as e:
        await update.callback_query.answer("API недоступен", show_alert=True)
        return
    if st != 200:
        await update.callback_query.answer(f"Ошибка API: {api_error_message(data)}", show_alert=True)
        return
    await update.callback_query.edit_message_caption(caption="✅ Оплата подтверждена. Подписка продлена.")
    try:
        await context.bot.send_message(chat_id=int(user_id), text="✅ Оплата подтверждена. Подписка продлена.")
    except Exception:
        pass

