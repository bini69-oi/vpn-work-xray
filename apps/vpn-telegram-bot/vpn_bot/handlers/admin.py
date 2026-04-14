from __future__ import annotations

import asyncio
import logging

from aiogram import F, Router
from aiogram.filters import Command, StateFilter
from aiogram.fsm.context import FSMContext
from aiogram.fsm.state import State, StatesGroup
from aiogram.enums import ParseMode
from aiogram.types import CallbackQuery, Message

from vpn_bot.config import settings
from vpn_bot.keyboards.admin_kb import admin_menu_kb, broadcast_confirm_kb, monitoring_kb
from vpn_bot.services.api_client import VPNApiClient
from vpn_bot.services.monitoring_service import format_health_report
from vpn_bot.utils import texts

log = logging.getLogger(__name__)

router = Router(name="admin")


class BroadcastStates(StatesGroup):
    waiting_text = State()
    preview = State()


def _is_admin(uid: int) -> bool:
    return uid in settings.admin_ids()


@router.message(Command("admin"))
async def cmd_admin(message: Message) -> None:
    uid = message.from_user.id if message.from_user else 0
    if not _is_admin(uid):
        await message.answer(texts.admin_only())
        return
    await message.answer(texts.admin_menu_text(), reply_markup=admin_menu_kb())


@router.callback_query(F.data == "admin_menu")
async def cb_admin_menu(query: CallbackQuery) -> None:
    await query.answer()
    if not query.from_user or not _is_admin(query.from_user.id):
        return
    if query.message:
        await query.message.edit_text(texts.admin_menu_text(), reply_markup=admin_menu_kb())


@router.callback_query(F.data == "mon_refresh")
async def cb_mon_refresh(query: CallbackQuery, api: VPNApiClient | None) -> None:
    await query.answer()
    if not query.from_user or not _is_admin(query.from_user.id):
        return
    if api is None or not query.message:
        await query.message.answer(texts.service_unavailable()) if query.message else None
        return
    st, data = await api.get_health()
    body = format_health_report(st, data if isinstance(data, dict) else {"data": data})
    if query.message:
        await query.message.edit_text(body, reply_markup=monitoring_kb())


@router.callback_query(F.data == "bc_start")
async def cb_bc_start(query: CallbackQuery, state: FSMContext) -> None:
    if not query.from_user or not _is_admin(query.from_user.id):
        await query.answer("Нет прав", show_alert=True)
        return
    await query.answer()
    await state.set_state(BroadcastStates.waiting_text)
    if query.message:
        await query.message.edit_text(texts.broadcast_ask_text())


@router.message(StateFilter(BroadcastStates.waiting_text), F.text, ~F.text.startswith("/"))
async def bc_capture(message: Message, state: FSMContext) -> None:
    if not message.from_user or not _is_admin(message.from_user.id):
        await state.clear()
        return
    text = message.html_text if getattr(message, "html_text", None) else (message.text or "")
    await state.update_data(bc_text=text, bc_entities=bool(message.entities))
    await state.set_state(BroadcastStates.preview)
    await message.answer(texts.broadcast_preview(text), reply_markup=broadcast_confirm_kb())


@router.callback_query(F.data == "broadcast_cancel", StateFilter(BroadcastStates.preview))
async def cb_bc_cancel(query: CallbackQuery, state: FSMContext) -> None:
    await query.answer()
    await state.clear()
    if query.message:
        await query.message.edit_text("Рассылка отменена.")


@router.callback_query(F.data == "broadcast_confirm", StateFilter(BroadcastStates.preview))
async def cb_bc_confirm(query: CallbackQuery, state: FSMContext, db) -> None:
    await query.answer()
    if not query.from_user or not _is_admin(query.from_user.id) or not query.message:
        return
    data = await state.get_data()
    html = str(data.get("bc_text") or "")
    use_html = bool(data.get("bc_entities"))
    await state.clear()
    cur = await db.execute("SELECT telegram_id FROM users WHERE is_banned = 0")
    rows = await cur.fetchall()
    targets = [int(r[0]) for r in rows]
    sent = 0
    failed = 0
    for uid in targets:
        try:
            if use_html:
                await query.bot.send_message(uid, html, parse_mode=ParseMode.HTML)
            else:
                await query.bot.send_message(uid, html)
            sent += 1
        except Exception as e:
            log.debug("broadcast to %s: %s", uid, e)
            failed += 1
        await asyncio.sleep(0.05)
    await query.message.edit_text(texts.broadcast_result(sent, failed, sent + failed))


@router.callback_query(F.data == "admin_close")
async def cb_admin_close(query: CallbackQuery, state: FSMContext) -> None:
    await state.clear()
    await query.answer()
    if query.message:
        await query.message.edit_text("Закрыто.")
