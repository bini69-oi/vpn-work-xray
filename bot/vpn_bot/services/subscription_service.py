from __future__ import annotations

from typing import Any

from vpn_bot.services.api_client import VPNApiClient


def vpn_user_id(telegram_user_id: int) -> str:
    return f"tg_{telegram_user_id}"


def delivery_profile_id(telegram_user_id: int) -> str:
    return f"user-tg-{telegram_user_id}"


async def user_has_issued_subscription(api: VPNApiClient | None, telegram_user_id: int) -> bool:
    """В vpn-productd есть выдача (subscriptionId), даже если срок истёк."""
    if api is None:
        return False
    st, status, _sub = await fetch_subscription_bundle(api, telegram_user_id)
    if st != 200 or not status:
        return False
    sid = str((status or {}).get("subscriptionId") or "").strip()
    return bool(sid)


async def resolve_reply_main_menu(api: VPNApiClient | None, telegram_user_id: int):
    """Нижнее меню: гость (4 кнопки без Мой VPN) или подписчик."""
    from vpn_bot.keyboards.main_menu import reply_main_menu_guest, reply_main_menu_subscribed

    if await user_has_issued_subscription(api, telegram_user_id):
        return reply_main_menu_subscribed()
    return reply_main_menu_guest()


async def fetch_subscription_bundle(api: VPNApiClient, telegram_user_id: int) -> tuple[int, dict[str, Any] | None, dict[str, Any] | None]:
    uid = vpn_user_id(telegram_user_id)
    st, status = await api.issue_status(uid)
    if st != 200:
        return st, None, status
    sub_id = str((status or {}).get("subscriptionId") or "").strip()
    if not sub_id:
        return st, status, None
    st2, sub = await api.get_subscription(sub_id)
    if st2 != 200:
        return st2, status, sub
    return 200, status, sub
