from __future__ import annotations

from typing import Any

from aiogram import F, Router
from aiogram.types import CallbackQuery, Message

from vpn_bot.keyboards.subscription_kb import happ_import_kb, renew_hint_kb, subscription_panel_kb
from vpn_bot.services.api_client import VPNApiClient
from vpn_bot.services.subscription_service import (
    delivery_profile_id,
    fetch_subscription_bundle,
    resolve_reply_main_menu,
    vpn_user_id,
)
from vpn_bot.utils import texts
from vpn_bot.utils.formatting import days_left, format_bytes, format_date_ru

router = Router(name="subscription")

# Лимит длины URL для кнопки Telegram
_TG_URL_MAX = 2048


def _find_user_profile_stats(stats: dict[str, Any], profile_id: str) -> tuple[int, int] | None:
    items = stats.get("items")
    if not isinstance(items, list):
        return None
    for it in items:
        if not isinstance(it, dict):
            continue
        if str(it.get("profileId", "")) != profile_id:
            continue
        up = int(it.get("uploadBytes") or 0)
        down = int(it.get("downloadBytes") or 0)
        total = int(it.get("totalBytes") or (up + down))
        limit_mb = int(it.get("trafficLimitMb") or 0)
        limit = limit_mb * 1024 * 1024 if limit_mb > 0 else 1024 * 1024 * 1024 * 1024
        return total, limit
    return None


def _pick_happ_import_link(links: dict[str, Any]) -> str | None:
    for _k, v in links.items():
        if not isinstance(v, str) or not v.strip():
            continue
        s = v.strip()
        if s.startswith(("vless://", "h2://", "vmess://", "trojan://", "ss://")):
            return s
    return None


def _happ_add_url(import_link: str) -> str:
    # # во vless-ссылке ломает разбор URL у клиентов — экранируем только решётку
    safe = import_link.replace("#", "%23")
    return "happ://add/" + safe


@router.message(F.text == "🛡 Мой VPN")
async def my_vpn(message: Message, api: VPNApiClient | None) -> None:
    if api is None:
        await message.answer(texts.service_unavailable())
        return
    uid = message.from_user.id if message.from_user else 0
    st, status, sub = await fetch_subscription_bundle(api, uid)
    if st != 200:
        await message.answer(texts.service_unavailable())
        return
    sid = str((status or {}).get("subscriptionId") or "").strip() if status else ""
    if st == 404 or not status or not sid:
        kb = await resolve_reply_main_menu(api, uid)
        await message.answer(texts.no_subscription_text(), reply_markup=kb)
        return

    sub_obj = sub or {}
    exp = str(sub_obj.get("expiresAt") or "").strip()
    dl = days_left(exp)
    active = dl > 0
    status_label = "✅ <b>Активна</b>" if active else "⏱ <b>Истекла / нет оплаты</b>"
    plan_label = "💎 <b>VIP</b>"

    total_used = 0
    total_limit = 1024 * 1024 * 1024 * 1024
    pid = delivery_profile_id(uid)
    pst, pdata = await api.get_profile_stats()
    if pst == 200:
        found = _find_user_profile_stats(pdata, pid)
        if found:
            total_used, total_limit = found

    devices_line = "📱 Данные по лимиту устройств — в панели сервера."
    body = texts.profile_text(
        status_label=status_label,
        plan_label=plan_label,
        expires_iso=exp or None,
        traffic_used=format_bytes(total_used),
        traffic_limit="∞ безлимит" if total_limit >= 1024**4 else format_bytes(total_limit),
        devices_line=devices_line,
    )
    await message.answer(body, reply_markup=subscription_panel_kb(has_active_subscription=active))
    await message.answer(texts.main_reply_hint(), reply_markup=await resolve_reply_main_menu(api, uid))


@router.callback_query(F.data == "sub_happ")
async def cb_sub_happ(query: CallbackQuery, api: VPNApiClient | None) -> None:
    await query.answer()
    if api is None or not query.from_user or not query.message:
        return
    uid = query.from_user.id
    pid = delivery_profile_id(uid)
    st, data = await api.get_delivery_links(pid)
    if st != 200:
        await query.answer("Не удалось получить данные. Попробуй позже.", show_alert=True)
        return
    raw = data.get("links")
    items: dict[str, Any] = raw if isinstance(raw, dict) else {}
    import_link = _pick_happ_import_link(items)
    if not import_link:
        await query.answer("Конфиг для HApp не готов. Напиши в поддержку.", show_alert=True)
        return
    happ = _happ_add_url(import_link)
    if len(happ) > _TG_URL_MAX:
        await query.answer("Слишком длинная ссылка для кнопки. Напиши в поддержку.", show_alert=True)
        return
    await query.message.edit_text(
        texts.happ_connect_instructions(),
        reply_markup=happ_import_kb(happ),
    )


@router.callback_query(F.data == "sub_renew_hint")
async def cb_renew_hint(query: CallbackQuery) -> None:
    await query.answer()
    if query.message:
        await query.message.edit_text(
            texts.renew_hint_text(),
            reply_markup=renew_hint_kb(),
        )


@router.callback_query(F.data == "sub_back_profile")
async def cb_sub_back(query: CallbackQuery, api: VPNApiClient | None) -> None:
    await query.answer()
    if not query.message or not query.from_user or api is None:
        return
    uid = query.from_user.id
    st, status, sub = await fetch_subscription_bundle(api, uid)
    if st != 200 or not sub:
        await query.message.edit_text(texts.no_subscription_text())
        return
    exp = str(sub.get("expiresAt") or "").strip()
    dl = days_left(exp)
    active = dl > 0
    body = texts.profile_text(
        status_label="✅ <b>Активна</b>" if active else "⏱ <b>Истекла / нет оплаты</b>",
        plan_label="💎 <b>VIP</b>",
        expires_iso=exp or None,
        traffic_used="—",
        traffic_limit="∞ безлимит",
        devices_line="📱 См. панель сервера.",
    )
    await query.message.edit_text(body, reply_markup=subscription_panel_kb(has_active_subscription=active))
