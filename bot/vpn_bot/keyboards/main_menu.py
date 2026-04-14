from __future__ import annotations

from aiogram.types import KeyboardButton, ReplyKeyboardMarkup


def reply_main_menu_guest() -> ReplyKeyboardMarkup:
    """Без «Мой VPN» и без ссылок — 4 кнопки, пока в API нет выданной подписки."""
    return ReplyKeyboardMarkup(
        keyboard=[
            [
                KeyboardButton(text="💎 Оплатить подписку"),
                KeyboardButton(text="🎁 Друзьям"),
            ],
            [
                KeyboardButton(text="💬 Помощь"),
                KeyboardButton(text="📋 О VPN-32"),
            ],
        ],
        resize_keyboard=True,
    )


def reply_main_menu_subscribed() -> ReplyKeyboardMarkup:
    """С «Мой VPN», без кнопки «Ссылки»."""
    return ReplyKeyboardMarkup(
        keyboard=[
            [
                KeyboardButton(text="🛡 Мой VPN"),
                KeyboardButton(text="💎 Оплатить подписку"),
            ],
            [
                KeyboardButton(text="🎁 Друзьям"),
                KeyboardButton(text="💬 Помощь"),
            ],
        ],
        resize_keyboard=True,
    )


def reply_main_menu() -> ReplyKeyboardMarkup:
    """Обратная совместимость: по умолчанию как у подписчика."""
    return reply_main_menu_subscribed()
