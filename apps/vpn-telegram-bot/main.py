"""Точка входа для совместимости. Предпочтительно: cd apps/vpn-telegram-bot && python -m vpn_bot."""

from __future__ import annotations

import asyncio

from vpn_bot.app import main

if __name__ == "__main__":
    asyncio.run(main())
