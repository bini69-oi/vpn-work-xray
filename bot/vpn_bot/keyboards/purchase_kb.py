from __future__ import annotations

from aiogram.types import InlineKeyboardButton, InlineKeyboardMarkup

from vpn_bot.config import settings
from vpn_bot.utils.formatting import PRICING


def plans_keyboard() -> InlineKeyboardMarkup:
    rows: list[list[InlineKeyboardButton]] = []
    for m in (1, 2, 3, 6, 12):
        p = PRICING[m]
        label = f"📦 {p['months']} мес. — {p['price']} ₽"
        if m == 6:
            label = f"🔥 {p['months']} мес. — {p['price']} ₽ (-{p['discount']}%)"
        rows.append([InlineKeyboardButton(text=label, callback_data=f"plan_{m}")])
    rows.append([InlineKeyboardButton(text="← Главное меню", callback_data="back_main")])
    return InlineKeyboardMarkup(inline_keyboard=rows)


def sbp_pay_flow_keyboard(months: int) -> InlineKeyboardMarkup:
    rows: list[list[InlineKeyboardButton]] = []
    if (settings.sbp_pay_url or "").strip():
        rows.append(
            [InlineKeyboardButton(text="💳 Перейти к оплате", url=(settings.sbp_pay_url or "").strip())],
        )
    else:
        rows.append(
            [InlineKeyboardButton(text="💳 Оплата (ссылка — заглушка)", callback_data=f"sbp_pay_stub_{months}")],
        )
    rows.append([InlineKeyboardButton(text="✅ Я оплатил", callback_data=f"confirm_paid_{months}")])
    rows.append([InlineKeyboardButton(text="← Назад", callback_data=f"back_pay_{months}")])
    return InlineKeyboardMarkup(inline_keyboard=rows)


def pay_methods_keyboard(months: int) -> InlineKeyboardMarkup:
    rows: list[list[InlineKeyboardButton]] = [
        [InlineKeyboardButton(text="💠 Оплатить по СБП", callback_data=f"pay_sbp_{months}")],
    ]
    if settings.payment_provider_token.strip():
        rows.append(
            [InlineKeyboardButton(text="💳 Оплатить картой (Telegram)", callback_data=f"pay_card_{months}")]
        )
    rows.append([InlineKeyboardButton(text="← К тарифам", callback_data="back_plans")])
    return InlineKeyboardMarkup(inline_keyboard=rows)


def confirm_paid_keyboard(months: int) -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="✅ Я оплатил", callback_data=f"confirm_paid_{months}")],
            [InlineKeyboardButton(text="← Назад", callback_data=f"back_pay_{months}")],
        ]
    )


def admin_payment_keyboard(payment_id: int) -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [
                InlineKeyboardButton(text="✅ Подтвердить", callback_data=f"admin_confirm_{payment_id}"),
                InlineKeyboardButton(text="❌ Отклонить", callback_data=f"admin_reject_{payment_id}"),
            ],
        ]
    )
