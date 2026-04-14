from __future__ import annotations

from aiogram.types import InlineKeyboardButton, InlineKeyboardMarkup


def happ_import_kb(happ_url: str) -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="📲 Открыть в HApp", url=happ_url)],
            [InlineKeyboardButton(text="← Назад к профилю", callback_data="sub_back_profile")],
        ]
    )


def subscription_panel_kb(*, has_active_subscription: bool) -> InlineKeyboardMarkup:
    if has_active_subscription:
        return InlineKeyboardMarkup(
            inline_keyboard=[
                [InlineKeyboardButton(text="📲 Подключить (HApp)", callback_data="sub_happ")],
                [InlineKeyboardButton(text="🔄 Продлить", callback_data="sub_renew_hint")],
                [InlineKeyboardButton(text="← Закрыть", callback_data="back_main")],
            ]
        )
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="💎 Оплатить подписку", callback_data="open_purchase_plans")],
            [InlineKeyboardButton(text="← Закрыть", callback_data="back_main")],
        ]
    )


def renew_hint_kb() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="💎 Выбрать тариф", callback_data="open_purchase_plans")],
            [InlineKeyboardButton(text="← Назад", callback_data="sub_back_profile")],
        ]
    )
