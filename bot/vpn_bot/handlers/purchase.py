from __future__ import annotations

import json
import logging
import time
from aiogram import F, Router
from aiogram.types import CallbackQuery, LabeledPrice, Message, PreCheckoutQuery

from vpn_bot.config import settings
from vpn_bot.services.subscription_service import resolve_reply_main_menu
from vpn_bot.keyboards.purchase_kb import (
    admin_payment_keyboard,
    pay_methods_keyboard,
    plans_keyboard,
    sbp_pay_flow_keyboard,
)
from vpn_bot.services.api_client import VPNApiClient
from vpn_bot.services.payment_service import confirm_payment, create_pending_payment
from vpn_bot.services.referral_service import finalize_referral_after_payment, get_referrer_for_payment_bonus
from vpn_bot.services.subscription_service import vpn_user_id
from vpn_bot.utils import texts
from vpn_bot.utils.formatting import PRICING

log = logging.getLogger(__name__)

router = Router(name="purchase")


async def _apply_paid_months(api: VPNApiClient, telegram_user_id: int, months: int) -> tuple[bool, str]:
    uid = vpn_user_id(telegram_user_id)
    st, _ = await api.issue_status(uid)
    if st == 404:
        st2, body = await api.issue_link(
            uid,
            "telegram-user",
            "telegram_aiogram_bot",
            settings.profile_ids(),
            f"aiogram-pay-{telegram_user_id}-{months}-{int(time.time())}",
        )
        if st2 != 200:
            return False, str(body.get("message") or body.get("error") or body)[:400]
        extra = months * 30 - 30
        if extra > 0:
            st3, b3 = await api.lifecycle_renew(uid, extra)
            if st3 != 200:
                return False, str(b3.get("message") or b3.get("error") or b3)[:400]
        return True, ""
    st4, b4 = await api.lifecycle_renew(uid, months * 30)
    if st4 != 200:
        return False, str(b4.get("message") or b4.get("error") or b4)[:400]
    return True, ""


@router.message(F.text.in_({"💎 Оплатить подписку", "💎 Купить"}))
async def open_purchase(message: Message) -> None:
    await message.answer(texts.pricing_text(), reply_markup=plans_keyboard())


@router.callback_query(F.data == "open_purchase_plans")
async def cb_open_purchase_plans(query: CallbackQuery) -> None:
    await query.answer()
    if query.message:
        await query.message.answer(texts.pricing_text(), reply_markup=plans_keyboard())
    elif query.from_user:
        await query.bot.send_message(query.from_user.id, texts.pricing_text(), reply_markup=plans_keyboard())


@router.callback_query(F.data.startswith("plan_"))
async def cb_plan(query: CallbackQuery) -> None:
    await query.answer()
    if not query.message or not query.data:
        return
    try:
        months = int(query.data.split("_", 1)[1])
    except ValueError:
        return
    p = PRICING.get(months)
    if not p:
        return
    await query.message.edit_text(
        texts.pay_method_text(months, p["price"]),
        reply_markup=pay_methods_keyboard(months),
    )


async def _open_sbp_payment_screen(query: CallbackQuery, months: int, api) -> None:
    await query.answer()
    if not query.message or not query.from_user:
        return
    p = PRICING.get(months)
    if not p:
        return
    has_url = bool((settings.sbp_pay_url or "").strip())
    await query.message.edit_text(
        texts.sbp_payment_page_text(months, p["price"], has_url),
        reply_markup=sbp_pay_flow_keyboard(months),
    )
    kb = await resolve_reply_main_menu(api, query.from_user.id)
    await query.message.answer(texts.sbp_reply_menu_hint(), reply_markup=kb)


@router.callback_query(F.data.startswith("pay_sbp_"))
async def cb_pay_sbp(query: CallbackQuery, api) -> None:
    if not query.data:
        return
    months = int(query.data.rsplit("_", 1)[-1])
    await _open_sbp_payment_screen(query, months, api)


@router.callback_query(F.data.startswith("pay_manual_"))
async def cb_pay_manual_legacy(query: CallbackQuery, api) -> None:
    """Старые сообщения с callback pay_manual_*."""
    if not query.data:
        return
    months = int(query.data.rsplit("_", 1)[-1])
    await _open_sbp_payment_screen(query, months, api)


@router.callback_query(F.data.startswith("sbp_pay_stub_"))
async def cb_sbp_pay_stub(query: CallbackQuery) -> None:
    await query.answer(
        "Заглушка: в bot/.env задай SBP_PAY_URL=https://… (страница банка или платёжной формы).",
        show_alert=True,
    )


@router.callback_query(F.data.startswith("pay_card_"))
async def cb_pay_card(query: CallbackQuery) -> None:
    await query.answer()
    if not query.message or not query.from_user or not query.data:
        return
    months = int(query.data.rsplit("_", 1)[-1])
    if not settings.payment_provider_token.strip():
        await query.message.edit_text(
            "💳 <b>Оплата картой</b>\n\n<i>Telegram Payments не настроены. Используй «Оплатить по СБП».</i>",
            reply_markup=pay_methods_keyboard(months),
        )
        return
    p = PRICING[months]
    prices = [LabeledPrice(label=f"VPN {p['months']} мес.", amount=int(p["price"]) * 100)]
    payload = json.dumps({"m": months, "u": query.from_user.id, "t": int(time.time())})
    await query.bot.send_invoice(
        chat_id=query.from_user.id,
        title="VPN подписка",
        description=f"Оплата: {p['months']} мес.",
        payload=payload[:128],
        provider_token=settings.payment_provider_token,
        currency=settings.payment_currency,
        prices=prices,
    )


@router.pre_checkout_query()
async def pre_checkout(pq: PreCheckoutQuery) -> None:
    await pq.answer(ok=True)


@router.message(F.successful_payment)
async def on_successful_payment(message: Message, api: VPNApiClient | None, db) -> None:
    sp = message.successful_payment
    if not sp or not message.from_user or api is None:
        return
    try:
        data = json.loads(sp.invoice_payload)
        months = int(data.get("m", 1))
    except Exception:
        months = 1
    ok, err = await _apply_paid_months(api, message.from_user.id, months)
    if not ok:
        await message.answer(f"⚠️ Оплата получена, но API: <code>{err}</code>")
        return
    ref = await get_referrer_for_payment_bonus(db, message.from_user.id)
    if ref:
        await api.lifecycle_renew(vpn_user_id(ref), settings.referral_bonus_days)
        await finalize_referral_after_payment(db, message.from_user.id)
        try:
            await message.bot.send_message(ref, texts.referral_friend_paid(settings.referral_bonus_days))
        except Exception as e:
            log.debug("referrer notify: %s", e)
    kb = await resolve_reply_main_menu(api, message.from_user.id)
    await message.answer(texts.payment_confirmed_user("Проверь «🛡 Мой VPN»."), reply_markup=kb)


@router.callback_query(F.data.startswith("confirm_paid_"))
async def cb_confirm_paid(query: CallbackQuery, db, api) -> None:
    await query.answer()
    if not query.message or not query.from_user or not query.data:
        return
    months = int(query.data.rsplit("_", 2)[-1])
    if settings.payment_stub_enabled:
        body = (
            texts.stub_payment_ok()
            if settings.payment_stub_result == "ok"
            else texts.stub_payment_fail()
        )
        await query.message.edit_text(body, reply_markup=None)
        kb = await resolve_reply_main_menu(api, query.from_user.id)
        await query.bot.send_message(query.from_user.id, texts.sbp_reply_menu_hint(), reply_markup=kb)
        return
    price = PRICING[months]["price"]
    pid = await create_pending_payment(db, query.from_user.id, months, price, "manual")
    user_label = f"@{query.from_user.username}" if query.from_user.username else str(query.from_user.id)
    for aid in settings.admin_ids():
        try:
            await query.bot.send_message(
                aid,
                texts.payment_pending_admin(user_label, months, price, pid),
                reply_markup=admin_payment_keyboard(pid),
            )
        except Exception as e:
            log.warning("admin notify %s: %s", aid, e)
    await query.message.edit_text(
        "✅ <b>Заявка отправлена</b>\n\nАдминистратор подтвердит оплату в ближайшее время.",
        reply_markup=None,
    )


@router.callback_query(F.data.startswith("admin_confirm_"))
async def cb_admin_confirm(query: CallbackQuery, api: VPNApiClient | None, db) -> None:
    if not query.from_user or query.from_user.id not in settings.admin_ids():
        await query.answer("Нет прав", show_alert=True)
        return
    await query.answer()
    if api is None or not query.data or not query.message:
        return
    pid = int(query.data.rsplit("_", 2)[-1])
    cur = await db.execute(
        "SELECT user_telegram_id, months FROM payments WHERE id = ? AND status = 'pending'",
        (pid,),
    )
    row = await cur.fetchone()
    if not row:
        await query.message.edit_text("Заявка не найдена или уже обработана.")
        return
    buyer = int(row[0])
    months = int(row[1])
    ok, err = await _apply_paid_months(api, buyer, months)
    if not ok:
        await query.message.edit_text(f"❌ API: <code>{err}</code>")
        return
    await confirm_payment(db, pid, query.from_user.id)
    ref = await get_referrer_for_payment_bonus(db, buyer)
    if ref:
        await api.lifecycle_renew(vpn_user_id(ref), settings.referral_bonus_days)
        await finalize_referral_after_payment(db, buyer)
        try:
            await query.bot.send_message(ref, texts.referral_friend_paid(settings.referral_bonus_days))
        except Exception as e:
            log.debug("referrer notify: %s", e)
    try:
        kb_buyer = await resolve_reply_main_menu(api, buyer)
        await query.bot.send_message(buyer, texts.payment_confirmed_user("Проверь «🛡 Мой VPN»."), reply_markup=kb_buyer)
    except Exception as e:
        log.warning("notify buyer: %s", e)
    await query.message.edit_text("✅ <b>Оплата подтверждена</b>, пользователь уведомлён.")


@router.callback_query(F.data.startswith("admin_reject_"))
async def cb_admin_reject(query: CallbackQuery, db) -> None:
    await query.answer()
    if not query.from_user or query.from_user.id not in settings.admin_ids():
        return
    if not query.data or not query.message:
        return
    pid = int(query.data.rsplit("_", 2)[-1])
    await db.execute("UPDATE payments SET status = 'cancelled' WHERE id = ?", (pid,))
    await db.commit()
    await query.message.edit_text("❌ Заявка отклонена.")


@router.callback_query(F.data == "back_plans")
async def cb_back_plans(query: CallbackQuery) -> None:
    await query.answer()
    if query.message:
        await query.message.edit_text(texts.pricing_text(), reply_markup=plans_keyboard())


@router.callback_query(F.data.startswith("back_pay_"))
async def cb_back_pay(query: CallbackQuery) -> None:
    await query.answer()
    if not query.message or not query.data:
        return
    months = int(query.data.rsplit("_", 1)[-1])
    p = PRICING[months]
    await query.message.edit_text(
        texts.pay_method_text(months, p["price"]),
        reply_markup=pay_methods_keyboard(months),
    )


@router.callback_query(F.data == "back_main")
async def cb_back_main(query: CallbackQuery) -> None:
    await query.answer()
    if query.message:
        await query.message.edit_text("⌨️ Главное меню — кнопки внизу экрана.")
