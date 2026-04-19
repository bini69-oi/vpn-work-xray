"""Pytest configuration for vpn-telegram-bot.

We make sure the package is importable without running any legacy .env or
triggering pydantic `BOT_TOKEN is required` errors. Tests that need a full
`Settings` instance construct it themselves with explicit env vars.
"""
from __future__ import annotations

import os
import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

# `vpn_bot.config.settings` instantiates a module-level `settings = Settings()`
# at import time, which requires BOT_TOKEN. On CI there is no `.env` file next
# to the package, so we seed a dummy token before any test module is imported.
# Conftest files are loaded by pytest before collecting tests in the same dir,
# so this runs early enough to cover every import path.
os.environ.setdefault("BOT_TOKEN", "123:stub")


@pytest.fixture(autouse=True)
def _isolate_env(monkeypatch: pytest.MonkeyPatch) -> None:
    """Make every test deterministic: wipe the bot-related env, pretend there
    is no `.env` file next to the package, and set a stub BOT_TOKEN so
    `Settings()` can be constructed on demand.
    """
    for key in list(os.environ):
        if (
            key.startswith(("VPN_", "REMNAWAVE_"))
            or key in {
                "BOT_TOKEN",
                "TELEGRAM_BOT_TOKEN",
                "ADMIN_IDS",
                "ADMIN_TELEGRAM_IDS",
                "ALLOWED_TELEGRAM_IDS",
                "TELEGRAM_PROXY",
                "TELEGRAM_HTTPS_PROXY",
                "DATABASE_PATH",
                "PAYMENT_STUB",
                "PAYMENT_STUB_RESULT",
                "PAYMENT_PROVIDER_TOKEN",
                "PAYMENT_CURRENCY",
                "PAYMENT_MANUAL_DETAILS",
                "SBP_PAY_URL",
                "SBP_PAYMENT_URL",
                "SUPPORT_USERNAME",
                "BOT_USERNAME",
                "REFERRAL_BONUS_DAYS",
                "CRYPTO_WALLET_ADDRESS",
            }
        ):
            monkeypatch.delenv(key, raising=False)
    monkeypatch.setenv("BOT_TOKEN", "123:stub")
