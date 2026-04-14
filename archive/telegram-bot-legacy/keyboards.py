from __future__ import annotations

from telegram import InlineKeyboardButton, InlineKeyboardMarkup, WebAppInfo


def main_menu(is_admin: bool, miniapp_url: str = "") -> InlineKeyboardMarkup:
    rows: list[list[InlineKeyboardButton]] = []
    url = (miniapp_url or "").strip()
    if url:
        rows.append([InlineKeyboardButton("📱 Приложение", web_app=WebAppInfo(url))])
    rows.extend(
        [
            [InlineKeyboardButton("🔑 Подписка", callback_data="subscribe"), InlineKeyboardButton("📊 Статус", callback_data="status")],
            [InlineKeyboardButton("🔗 Ссылки", callback_data="links"), InlineKeyboardButton("💳 Оплата", callback_data="pay")],
            [InlineKeyboardButton("📋 История", callback_data="history"), InlineKeyboardButton("❓ Помощь", callback_data="help")],
        ]
    )
    if is_admin:
        rows.append([InlineKeyboardButton("⚙️ Админ-панель", callback_data="admin")])
    return InlineKeyboardMarkup(rows)


def subscribe_choice() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        [
            [InlineKeyboardButton("🔄 Продлить", callback_data="renew"), InlineKeyboardButton("🆕 Новая подписка", callback_data="subscribe_new")],
            [InlineKeyboardButton("Отмена", callback_data="cancel")],
        ]
    )


def pay_plans(plan_buttons: list[InlineKeyboardButton]) -> InlineKeyboardMarkup:
    rows: list[list[InlineKeyboardButton]] = []
    for btn in plan_buttons:
        rows.append([btn])
    rows.append([InlineKeyboardButton("Отмена", callback_data="cancel")])
    return InlineKeyboardMarkup(rows)


def manual_payment_confirm() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup([[InlineKeyboardButton("✅ Я оплатил", callback_data="manual_paid")]])


def admin_payment_confirm(user_id: int, months: int) -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup([[InlineKeyboardButton("✅ Подтвердить оплату", callback_data=f"admin_confirm_payment:{user_id}:{months}")]])


def back_to_menu() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup([[InlineKeyboardButton("⬅️ В меню", callback_data="menu")]])

