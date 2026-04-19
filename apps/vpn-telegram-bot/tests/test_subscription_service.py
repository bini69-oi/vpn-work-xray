"""Tests for `fetch_subscription_bundle` and related helpers."""
from __future__ import annotations

from typing import Any

from vpn_bot.services.subscription_service import (
    delivery_profile_id,
    fetch_subscription_bundle,
    user_has_issued_subscription,
    vpn_user_id,
)


class StubBackend:
    """Minimal in-memory VPNBackend for unit tests."""

    def __init__(
        self,
        *,
        status_resp: tuple[int, dict[str, Any]] = (404, {}),
        sub_resp: tuple[int, dict[str, Any]] = (200, {}),
    ) -> None:
        self.status_resp = status_resp
        self.sub_resp = sub_resp
        self.get_subscription_calls: list[str] = []

    async def issue_status(self, user_id: str):
        return self.status_resp

    async def issue_link(self, *a, **kw):
        return 200, {}

    async def lifecycle_renew(self, user_id: str, days: int):
        return 200, {}

    async def get_subscription(self, sid: str):
        self.get_subscription_calls.append(sid)
        return self.sub_resp

    async def get_delivery_links(self, pid: str):
        return 200, {}

    async def get_health(self):
        return 200, {}


class TestIds:
    def test_vpn_user_id(self) -> None:
        assert vpn_user_id(42) == "tg_42"

    def test_delivery_profile_id(self) -> None:
        assert delivery_profile_id(42) == "user-tg-42"


class TestFetchSubscriptionBundle:
    async def test_no_status(self) -> None:
        api = StubBackend(status_resp=(500, {}))
        st, status, sub = await fetch_subscription_bundle(api, 42)
        assert st == 500
        assert status is None
        assert api.get_subscription_calls == []

    async def test_status_without_subscription_id(self) -> None:
        api = StubBackend(status_resp=(200, {}))
        st, status, sub = await fetch_subscription_bundle(api, 42)
        assert st == 200
        assert sub is None

    async def test_full_bundle(self) -> None:
        api = StubBackend(
            status_resp=(200, {"subscriptionId": "abc"}),
            sub_resp=(200, {"id": "abc", "expiresAt": "2030-01-01T00:00:00Z"}),
        )
        st, status, sub = await fetch_subscription_bundle(api, 42)
        assert st == 200
        assert sub["id"] == "abc"
        assert api.get_subscription_calls == ["abc"]

    async def test_sub_lookup_failure_surfaced(self) -> None:
        api = StubBackend(
            status_resp=(200, {"subscriptionId": "abc"}),
            sub_resp=(500, {"message": "boom"}),
        )
        st, status, sub = await fetch_subscription_bundle(api, 42)
        assert st == 500


class TestUserHasIssuedSubscription:
    async def test_none_api(self) -> None:
        assert await user_has_issued_subscription(None, 42) is False

    async def test_no_status(self) -> None:
        api = StubBackend(status_resp=(404, {}))
        assert await user_has_issued_subscription(api, 42) is False

    async def test_has_subscription(self) -> None:
        api = StubBackend(
            status_resp=(200, {"subscriptionId": "abc"}),
            sub_resp=(200, {"id": "abc"}),
        )
        assert await user_has_issued_subscription(api, 42) is True
