from __future__ import annotations

from aiogram.types import InlineKeyboardButton, InlineKeyboardMarkup


def admin_menu_kb() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="📊 Мониторинг (health)", callback_data="mon_refresh")],
            [InlineKeyboardButton(text="📢 Рассылка", callback_data="bc_start")],
            [InlineKeyboardButton(text="← Закрыть", callback_data="admin_close")],
        ]
    )


def monitoring_kb() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="🔄 Обновить", callback_data="mon_refresh")],
            [InlineKeyboardButton(text="← Админка", callback_data="admin_menu")],
        ]
    )


def broadcast_confirm_kb() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [
                InlineKeyboardButton(text="✅ Отправить", callback_data="broadcast_confirm"),
                InlineKeyboardButton(text="❌ Отмена", callback_data="broadcast_cancel"),
            ],
        ]
    )
