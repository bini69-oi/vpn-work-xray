from __future__ import annotations

from aiogram import F, Router
from aiogram.types import CallbackQuery, InlineKeyboardButton, InlineKeyboardMarkup, Message

from vpn_bot.config import settings
from vpn_bot.services.api_client import VPNApiClient
from vpn_bot.services.subscription_service import resolve_reply_main_menu
from vpn_bot.utils import texts

router = Router(name="support")


def _faq_kb() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="📱 Как подключиться?", callback_data="faq_connect")],
            [InlineKeyboardButton(text="🔄 Не работает VPN", callback_data="faq_vpn")],
            [InlineKeyboardButton(text="💳 Вопросы по оплате", callback_data="faq_pay")],
            [InlineKeyboardButton(text="👤 Написать в поддержку", url=_support_url())],
            [InlineKeyboardButton(text="← Меню", callback_data="faq_back_menu")],
        ]
    )


def _support_url() -> str:
    u = settings.support_username.strip().lstrip("@")
    if u:
        return f"https://t.me/{u}"
    return "https://t.me/telegram"


@router.message(F.text == "💬 Помощь")
async def cmd_help_menu(message: Message) -> None:
    await message.answer(texts.help_menu_text(), reply_markup=_faq_kb())


@router.callback_query(F.data == "faq_connect")
async def cb_faq_connect(query: CallbackQuery) -> None:
    await query.answer()
    if query.message:
        await query.message.edit_text(texts.faq_connect(), reply_markup=_faq_back_kb())


@router.callback_query(F.data == "faq_vpn")
async def cb_faq_vpn(query: CallbackQuery) -> None:
    await query.answer()
    if query.message:
        await query.message.edit_text(texts.faq_not_working(), reply_markup=_faq_back_kb())


@router.callback_query(F.data == "faq_pay")
async def cb_faq_pay(query: CallbackQuery) -> None:
    await query.answer()
    if query.message:
        await query.message.edit_text(texts.faq_payment(), reply_markup=_faq_back_kb())


def _faq_back_kb() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[[InlineKeyboardButton(text="← Назад к FAQ", callback_data="faq_menu")]],
    )


@router.callback_query(F.data == "faq_menu")
async def cb_faq_menu(query: CallbackQuery) -> None:
    await query.answer()
    if query.message:
        await query.message.edit_text(texts.help_menu_text(), reply_markup=_faq_kb())


@router.callback_query(F.data == "faq_back_menu")
async def cb_faq_back_menu(query: CallbackQuery, api: VPNApiClient | None) -> None:
    await query.answer()
    if query.message:
        await query.message.delete()
    if query.from_user:
        kb = await resolve_reply_main_menu(api, query.from_user.id)
        await query.bot.send_message(query.from_user.id, texts.main_reply_hint(), reply_markup=kb)
