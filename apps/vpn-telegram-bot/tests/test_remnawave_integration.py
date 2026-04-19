"""Integration test for `RemnawaveApiClient` against a live aiohttp server.

This replaces the testcontainers plan (item 9 in the roadmap): rather than
pulling the real Remnawave Panel image (which requires Postgres, Redis, etc.
and is too heavy for CI), we spin up a local aiohttp app that mimics the
subset of the Remnawave Panel REST API the bot actually uses.

It verifies end-to-end wire behaviour: TCP connection, HTTP headers (Bearer
token, optional caddy X-Api-Key), JSON bodies, 404/200 status codes and the
shape the bot returns to its handlers.
"""
from __future__ import annotations

from datetime import UTC, datetime, timedelta
from typing import Any

import aiohttp
import pytest
from aiohttp import web
from aiohttp.test_utils import TestServer

from vpn_bot.services.remnawave_client import RemnawaveApiClient


def _make_user(telegram_id: int, *, uuid: str = "uuid-42") -> dict[str, Any]:
    return {
        "uuid": uuid,
        "username": f"tg{telegram_id}",
        "telegramId": telegram_id,
        "status": "ACTIVE",
        "subscriptionUrl": f"https://panel.test/sub/{uuid}",
        "expireAt": (datetime.now(UTC) + timedelta(days=10)).isoformat().replace("+00:00", "Z"),
        "trafficLimitBytes": 0,
        "userTraffic": {"usedTrafficBytes": 12345},
    }


class _FakePanel:
    """Tiny in-memory Remnawave Panel emulator (handlers the bot actually calls)."""

    def __init__(self) -> None:
        self.users: dict[int, dict[str, Any]] = {}
        self.received_auth: list[str] = []
        self.received_api_key: list[str] = []

    def app(self) -> web.Application:
        app = web.Application()
        app.router.add_get("/api/users/by-telegram-id/{tg}", self._by_tg)
        app.router.add_post("/api/users", self._create_user)
        app.router.add_patch("/api/users", self._patch_user)
        app.router.add_get("/api/users/{uuid}", self._get_user_by_uuid)
        app.router.add_get("/api/system/health", self._health)
        return app

    def _record_headers(self, request: web.Request) -> None:
        self.received_auth.append(request.headers.get("Authorization", ""))
        self.received_api_key.append(request.headers.get("X-Api-Key", ""))

    async def _by_tg(self, request: web.Request) -> web.Response:
        self._record_headers(request)
        tg = int(request.match_info["tg"])
        user = self.users.get(tg)
        if not user:
            return web.json_response({"message": "not found"}, status=404)
        return web.json_response({"response": user})

    async def _create_user(self, request: web.Request) -> web.Response:
        self._record_headers(request)
        body = await request.json()
        tg = int(body["telegramId"])
        user = _make_user(tg, uuid=f"uuid-{tg}")
        user["username"] = body.get("username", user["username"])
        user["expireAt"] = body.get("expireAt", user["expireAt"])
        self.users[tg] = user
        return web.json_response({"response": user}, status=201)

    async def _patch_user(self, request: web.Request) -> web.Response:
        self._record_headers(request)
        body = await request.json()
        uid = body["uuid"]
        for u in self.users.values():
            if u["uuid"] == uid:
                if "expireAt" in body:
                    u["expireAt"] = body["expireAt"]
                return web.json_response({"response": u})
        return web.json_response({"message": "not found"}, status=404)

    async def _get_user_by_uuid(self, request: web.Request) -> web.Response:
        self._record_headers(request)
        uid = request.match_info["uuid"]
        for u in self.users.values():
            if u["uuid"] == uid:
                return web.json_response({"response": u})
        return web.json_response({"message": "not found"}, status=404)

    async def _health(self, request: web.Request) -> web.Response:
        self._record_headers(request)
        return web.json_response({"status": "ok"})


class _PanelHarness:
    """Context manager that spins up a `_FakePanel` on an ephemeral localhost port
    and hands back (fake, client) ready to use."""

    def __init__(self, *, squads: list[str] | None = None, caddy: str = "") -> None:
        self.fake = _FakePanel()
        self._server = TestServer(self.fake.app())
        self._session: aiohttp.ClientSession | None = None
        self._squads = squads if squads is not None else ["squad-1"]
        self._caddy = caddy
        self.client: RemnawaveApiClient | None = None

    async def __aenter__(self) -> tuple[_FakePanel, RemnawaveApiClient]:
        await self._server.start_server()
        self._session = aiohttp.ClientSession()
        base = str(self._server.make_url("")).rstrip("/")
        self.client = RemnawaveApiClient(
            self._session,
            base,
            "integration-token",
            caddy_token=self._caddy,
            internal_squad_uuids=self._squads,
        )
        return self.fake, self.client

    async def __aexit__(self, *exc: Any) -> None:
        if self._session is not None:
            await self._session.close()
        await self._server.close()


@pytest.fixture
async def harness():
    async with _PanelHarness(caddy="caddy-xyz") as (fake, client):
        yield fake, client


class TestIntegrationFlow:
    async def test_full_subscription_lifecycle(self, harness) -> None:
        fake, client = harness

        st, body = await client.issue_status("tg_42")
        assert st == 404

        st, body = await client.issue_link("tg_42", "telegram-user", "telegram_aiogram_bot", None, None)
        assert st == 200
        assert body["subscription"]["id"] == "uuid-42"
        assert body["url"].startswith("https://panel.test/")

        st, body = await client.issue_status("tg_42")
        assert st == 200
        assert body["subscriptionId"] == "uuid-42"
        assert body["status"] == "verified"

        st, body = await client.lifecycle_renew("tg_42", days=30)
        assert st == 200 and body == {"ok": True}

        st, body = await client.get_subscription("uuid-42")
        assert st == 200
        assert body["id"] == "uuid-42"
        assert body["usedTrafficBytes"] == 12345

        st, body = await client.get_delivery_links("user-tg-42")
        assert st == 200
        assert body["links"]["subscription"].startswith("https://panel.test/")

        st, body = await client.get_health()
        assert st == 200
        assert body.get("status") == "ok"

        assert fake.received_auth
        assert all(h == "Bearer integration-token" for h in fake.received_auth)
        assert all(h == "caddy-xyz" for h in fake.received_api_key)

    async def test_404_for_unknown_profile(self, harness) -> None:
        _, client = harness
        st, _ = await client.get_delivery_links("user-tg-999")
        assert st == 404

    async def test_issue_link_requires_squads(self) -> None:
        async with _PanelHarness(squads=[]) as (_, client):
            st, body = await client.issue_link("tg_99", "x", "x", None, None)
            assert st == 503
            assert "SQUAD" in body["message"]
