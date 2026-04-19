"""Backend protocol used by handlers and services.

The bot is Remnawave-only since the cutover from `vpn-productd`. The Protocol
stays here so handlers depend on a narrow interface (and tests can swap in a
fake) instead of the concrete `RemnawaveApiClient`.

Re-exported for backwards compatibility with imports like
`from vpn_bot.services.api_client import VPNBackend, ApiError`.
"""
from __future__ import annotations

from typing import Any, Protocol


class ApiError(Exception):
    def __init__(self, status: int, body: str) -> None:
        super().__init__(f"HTTP {status}: {body[:500]}")
        self.status = status
        self.body = body


class VPNBackend(Protocol):
    """Узкий контракт, который handlers/services ожидают от любого бэкенда.

    Сейчас единственная реализация — `RemnawaveApiClient`. Сохраняем Protocol,
    чтобы тесты могли подменять fake-клиент без `mock.patch`-магии.
    """

    async def issue_status(self, user_id: str) -> tuple[int, dict[str, Any]]: ...

    async def issue_link(
        self,
        user_id: str,
        name: str,
        source: str,
        profile_ids: list[str] | None,
        idempotency_key: str | None,
    ) -> tuple[int, dict[str, Any]]: ...

    async def lifecycle_renew(self, user_id: str, days: int) -> tuple[int, dict[str, Any]]: ...

    async def get_subscription(self, subscription_id: str) -> tuple[int, dict[str, Any]]: ...

    async def get_delivery_links(self, profile_id: str) -> tuple[int, dict[str, Any]]: ...

    async def get_health(self) -> tuple[int, dict[str, Any]]: ...
