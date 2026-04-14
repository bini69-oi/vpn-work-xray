from __future__ import annotations

import httpx
from telegram import Update
from telegram.ext import ContextTypes

from handlers.common import api_client, api_error_message, deny_if_not_allowed, handle_request_error, reply_text
from keyboards import back_to_menu
from texts import NO_SUBSCRIPTION
from utils import days_left, format_bytes, format_date, progress_bar, user_id_for_api, user_profile_id


def _find_user_profile_stats(stats: dict, profile_id: str) -> tuple[int, int, int, bool] | None:
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
        blocked = bool(it.get("blocked") or False)
        return total, limit, limit_mb, blocked
    return None


async def cmd_status(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if await deny_if_not_allowed(update, context):
        return
    uid = update.effective_user.id
    client = api_client(context)
    if client is None:
        await reply_text(update, context, "Режим проверки без сервера (BOT_DRY_RUN=1).", reply_markup=back_to_menu())
        return
    api_uid = user_id_for_api(uid)
    pid = user_profile_id(uid)

    try:
        st, data = await client.issue_status(api_uid)
    except httpx.RequestError as e:
        await handle_request_error(update, context, e)
        return

    if st == 404:
        await reply_text(update, context, NO_SUBSCRIPTION, reply_markup=back_to_menu())
        return
    if st != 200:
        await reply_text(update, context, f"Ошибка API ({st}): {api_error_message(data)}", reply_markup=back_to_menu())
        return

    sub_id = str(data.get("subscriptionId") or "").strip()
    applied = data.get("appliedTo3XUI")

    expires_at = ""
    if sub_id:
        try:
            sst, sdata = await client.get_subscription(sub_id)
        except httpx.RequestError as e:
            await handle_request_error(update, context, e)
            return
        if sst == 200:
            sub = sdata.get("subscription") or {}
            expires_at = str(sub.get("expiresAt") or "").strip()

    total_used = 0
    total_limit = 1024 * 1024 * 1024 * 1024
    try:
        pst, pdata = await client.get_profile_stats()
        if pst == 200:
            found = _find_user_profile_stats(pdata, pid)
            if found:
                total_used, total_limit, _, blocked = found
    except httpx.RequestError:
        pass

    acc_status = "unknown"
    try:
        ast, adata = await client.get_account(pid)
        if ast == 200:
            acc = adata.get("account") or {}
            acc_status = str(acc.get("status") or "unknown")
    except httpx.RequestError:
        pass

    left = days_left(expires_at) if expires_at else 0
    active = acc_status == "active" and left > 0

    lines: list[str] = []
    if active:
        lines.append("✅ Подписка: активна")
        lines.append(f"📅 До: {format_date(expires_at)} (осталось {left} дней)")
    else:
        lines.append("❌ Подписка: неактивна")
        if expires_at:
            lines.append(f"📅 До: {format_date(expires_at)}")
        lines.append("Продлить: /renew")

    lines.append(f"📊 Трафик: {format_bytes(total_used)} / {format_bytes(total_limit)}")
    lines.append(progress_bar(total_used, total_limit))
    lines.append(f"🌐 Профиль: {pid}")
    if applied is True:
        lines.append("🔄 3x-ui: синхронизирован")
    elif applied is False:
        lines.append("🔄 3x-ui: не синхронизирован")

    await reply_text(update, context, "\n".join(lines), reply_markup=back_to_menu())

