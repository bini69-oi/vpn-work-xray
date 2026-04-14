from __future__ import annotations

import json
import os
from dataclasses import dataclass


@dataclass(frozen=True)
class PaymentPlan:
    months: int
    price_rub: int
    label: str


@dataclass(frozen=True)
class BotConfig:
    telegram_bot_token: str
    dry_run: bool

    vpn_product_base_url: str
    vpn_product_api_token: str
    vpn_profile_ids: list[str] | None

    allowed_telegram_ids: set[int] | None
    admin_telegram_ids: set[int]

    rate_limit_seconds: int

    payment_provider_token: str
    payment_currency: str
    payment_plans: list[PaymentPlan]
    payment_manual_details: str

    loki_url: str
    loki_tags: str
    loki_username: str
    loki_password: str

    # HTTPS URL of the Mini App (same host as Express in apps/telegram-miniapp/). Adds a WebApp button in the menu.
    miniapp_url: str


def _env_truthy(name: str) -> bool:
    v = os.environ.get(name, "").strip().lower()
    return v in ("1", "true", "yes", "on")


def _parse_ids(raw: str) -> set[int]:
    out: set[int] = set()
    for part in raw.split(","):
        part = part.strip()
        if not part:
            continue
        try:
            out.add(int(part))
        except ValueError:
            continue
    return out


def _parse_profile_ids(raw: str) -> list[str] | None:
    raw = raw.strip()
    if not raw:
        return None
    ids = [p.strip() for p in raw.split(",") if p.strip()]
    return ids if ids else None


def _default_plans() -> list[PaymentPlan]:
    return [
        PaymentPlan(months=1, price_rub=150, label="1 месяц"),
        PaymentPlan(months=3, price_rub=400, label="3 месяца"),
        PaymentPlan(months=6, price_rub=700, label="6 месяцев"),
        PaymentPlan(months=12, price_rub=1200, label="12 месяцев"),
    ]


def _parse_plans(raw: str) -> list[PaymentPlan]:
    raw = raw.strip()
    if not raw:
        return _default_plans()
    try:
        data = json.loads(raw)
    except Exception:
        return _default_plans()
    if not isinstance(data, list):
        return _default_plans()
    out: list[PaymentPlan] = []
    for it in data:
        if not isinstance(it, dict):
            continue
        months = int(it.get("months") or 0)
        price = int(it.get("price_rub") or 0)
        label = str(it.get("label") or "").strip()
        if months <= 0 or price <= 0 or not label:
            continue
        out.append(PaymentPlan(months=months, price_rub=price, label=label))
    return out if out else _default_plans()


def load_config() -> BotConfig:
    token = os.environ.get("TELEGRAM_BOT_TOKEN", "").strip()
    dry_run = _env_truthy("BOT_DRY_RUN") or _env_truthy("TELEGRAM_BOT_DRY_RUN")

    base_url = os.environ.get("VPN_PRODUCT_BASE_URL", "").strip()
    api_token = os.environ.get("VPN_PRODUCT_API_TOKEN", "").strip()
    profile_ids = _parse_profile_ids(os.environ.get("VPN_PROFILE_IDS", ""))

    allowed_raw = os.environ.get("ALLOWED_TELEGRAM_IDS", "").strip()
    allowed = _parse_ids(allowed_raw) if allowed_raw else None

    admin_raw = os.environ.get("ADMIN_TELEGRAM_IDS", "").strip()
    admins = _parse_ids(admin_raw) if admin_raw else set()

    rate_limit_seconds = int(os.environ.get("BOT_RATE_LIMIT_SECONDS", "60") or "60")
    if rate_limit_seconds <= 0:
        rate_limit_seconds = 60

    payment_provider_token = os.environ.get("PAYMENT_PROVIDER_TOKEN", "").strip()
    payment_currency = os.environ.get("PAYMENT_CURRENCY", "RUB").strip() or "RUB"
    payment_plans = _parse_plans(os.environ.get("PAYMENT_PLANS", ""))
    payment_manual_details = os.environ.get("PAYMENT_MANUAL_DETAILS", "").strip()

    return BotConfig(
        telegram_bot_token=token,
        dry_run=dry_run,
        vpn_product_base_url=base_url,
        vpn_product_api_token=api_token,
        vpn_profile_ids=profile_ids,
        allowed_telegram_ids=allowed,
        admin_telegram_ids=admins,
        rate_limit_seconds=rate_limit_seconds,
        payment_provider_token=payment_provider_token,
        payment_currency=payment_currency,
        payment_plans=payment_plans,
        payment_manual_details=payment_manual_details,
        loki_url=os.environ.get("LOKI_URL", "").strip(),
        loki_tags=os.environ.get("LOKI_TAGS", "").strip(),
        loki_username=os.environ.get("LOKI_USERNAME", "").strip(),
        loki_password=os.environ.get("LOKI_PASSWORD", "").strip(),
        miniapp_url=os.environ.get("TELEGRAM_MINIAPP_URL", "").strip(),
    )

