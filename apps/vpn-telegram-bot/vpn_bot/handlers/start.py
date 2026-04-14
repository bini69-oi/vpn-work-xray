from __future__ import annotations

import logging

from aiogram import F, Router
from aiogram.filters import CommandStart
from aiogram.types import Message

from vpn_bot.services.api_client import VPNApiClient
from vpn_bot.services.referral_service import record_referral
from vpn_bot.services.subscription_service import resolve_reply_main_menu
from vpn_bot.utils import texts

log = logging.getLogger(__name__)

router = Router(name="start")


@router.message(CommandStart())
async def cmd_start(message: Message, db, api: VPNApiClient | None) -> None:
    uid = message.from_user.id if message.from_user else 0
    parts = (message.text or "").split(maxsplit=1)
    payload = parts[1].strip() if len(parts) > 1 else ""
    if payload.startswith("ref_"):
        raw = payload[4:]
        if raw.isdigit():
            ref_id = int(raw)
            await record_referral(db, ref_id, uid)
            try:
                await message.bot.send_message(ref_id, texts.referral_friend_joined())
            except Exception as e:
                log.debug("notify referrer: %s", e)
    kb = await resolve_reply_main_menu(api, uid)
    await message.answer(texts.welcome_text(), reply_markup=kb)
    await message.answer(texts.main_reply_hint())


@router.message(F.text == "📋 О VPN-32")
async def cmd_about_vpn(message: Message, api: VPNApiClient | None) -> None:
    uid = message.from_user.id if message.from_user else 0
    kb = await resolve_reply_main_menu(api, uid)
    await message.answer(texts.about_vpn_short(), reply_markup=kb)
