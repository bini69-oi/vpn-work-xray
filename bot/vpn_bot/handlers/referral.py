from __future__ import annotations

from aiogram import Dispatcher, F, Router
from aiogram.types import CallbackQuery, Message

from vpn_bot.config import settings
from vpn_bot.keyboards.referral_kb import referral_invite_card_kb, referral_keyboard
from vpn_bot.services.referral_service import referral_stats
from vpn_bot.utils import texts

router = Router(name="referral")


def _resolve_bot_username(dispatcher: Dispatcher | None) -> str:
    if settings.bot_username.strip():
        return settings.bot_username.strip()
    if dispatcher is not None:
        try:
            v = dispatcher["bot_username"]
            if v:
                return str(v)
        except Exception:
            pass
    return ""


@router.message(F.text == "🎁 Друзьям")
async def cmd_referral(message: Message, db, dispatcher: Dispatcher) -> None:
    uid = message.from_user.id if message.from_user else 0
    uname = _resolve_bot_username(dispatcher)
    if not uname:
        await message.answer("Задайте BOT_USERNAME в .env или дождитесь запуска бота.")
        return
    invited, paid = await referral_stats(db, uid)
    body = texts.referral_text(uname, uid, invited, paid, settings.referral_bonus_days)
    await message.answer(body, reply_markup=referral_keyboard())


@router.callback_query(F.data == "referral_fwd")
async def cb_referral_forward_pack(query: CallbackQuery, dispatcher: Dispatcher) -> None:
    await query.answer()
    uid = query.from_user.id if query.from_user else 0
    chat_id = query.message.chat.id if query.message else (query.from_user.id if query.from_user else 0)
    uname = _resolve_bot_username(dispatcher)
    if not chat_id:
        return
    if not uname:
        await query.bot.send_message(chat_id, "Задайте BOT_USERNAME в .env или дождитесь запуска бота.")
        return
    link = f"https://t.me/{uname}?start=ref_{uid}"
    await query.bot.send_message(
        chat_id,
        texts.referral_invite_friend_card(),
        reply_markup=referral_invite_card_kb(link),
    )
