from __future__ import annotations

from aiogram.types import InlineKeyboardButton, InlineKeyboardMarkup


def referral_invite_card_kb(link: str) -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="🚀 Открыть VPN-32", url=link)],
        ]
    )


def referral_keyboard() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="📨 Отправить сообщение", callback_data="referral_fwd")],
            [InlineKeyboardButton(text="← Главное меню", callback_data="back_main")],
        ]
    )
