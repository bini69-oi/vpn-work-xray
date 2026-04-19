"""End-to-end unit tests for `RemnawaveApiClient` using a fake aiohttp session.

The goal is to lock down the *wire format* we send to the Remnawave Panel REST
API (paths, headers, request bodies) and the *shape* we return to the handlers
via the `VPNBackend` Protocol contract.
"""
from __future__ import annotations

import json
from datetime import UTC, datetime, timedelta

import pytest

from tests._fake_session import FakeSession
from vpn_bot.services.api_client import VPNBackend
from vpn_bot.services.remnawave_client import ApiError, RemnawaveApiClient


def _make_client(
    session: FakeSession,
    *,
    panel_url: str = "https://panel.example.com",
    api_token: str = "test-token",
    caddy_token: str = "",
    squads: list[str] | None = None,
) -> RemnawaveApiClient:
    return RemnawaveApiClient(
        session,  # type: ignore[arg-type]
        panel_url,
        api_token,
        caddy_token=caddy_token,
        internal_squad_uuids=squads,
    )


class TestHeadersAndBaseUrl:
    def test_bearer_header_is_built(self) -> None:
        c = _make_client(FakeSession(), api_token="abc")
        assert c._headers["Authorization"] == "Bearer abc"
        assert c._headers["Content-Type"] == "application/json"

    def test_bearer_already_prefixed_is_kept(self) -> None:
        c = _make_client(FakeSession(), api_token="Bearer abc")
        assert c._headers["Authorization"] == "Bearer abc"

    def test_caddy_token_adds_x_api_key(self) -> None:
        c = _make_client(FakeSession(), caddy_token="caddy-xyz")
        assert c._headers["X-Api-Key"] == "caddy-xyz"

    def test_http_panel_adds_forwarded_proto(self) -> None:
        c = _make_client(FakeSession(), panel_url="http://panel.local:3000")
        assert c._headers["x-forwarded-proto"] == "https"
        assert c._headers["x-forwarded-for"] == "127.0.0.1"

    def test_https_panel_does_not_add_forwarded_proto(self) -> None:
        c = _make_client(FakeSession(), panel_url="https://panel.local")
        assert "x-forwarded-proto" not in c._headers

    def test_normalized_root(self) -> None:
        c = _make_client(FakeSession(), panel_url="https://panel.local/")
        assert c._root == "https://panel.local/api"

    def test_panel_url_already_has_api(self) -> None:
        c = _make_client(FakeSession(), panel_url="https://panel.local/api")
        assert c._root == "https://panel.local/api"


class TestIssueStatus:
    async def test_returns_200_verified_when_user_is_active(self) -> None:
        s = FakeSession()
        s.on(
            "GET",
            "/api/users/by-telegram-id/42",
            200,
            json.dumps({"response": {"uuid": "uuid-1", "status": "ACTIVE"}}),
        )
        c = _make_client(s)

        status, body = await c.issue_status("tg_42")

        assert status == 200
        assert body["subscriptionId"] == "uuid-1"
        assert body["status"] == "verified"
        assert body["userId"] == "tg_42"

    async def test_returns_404_when_user_not_found(self) -> None:
        s = FakeSession()
        s.on("GET", "/api/users/by-telegram-id/42", 404, "{}")
        c = _make_client(s)

        status, body = await c.issue_status("tg_42")

        assert status == 404
        assert "message" in body

    async def test_returns_400_for_invalid_user_id(self) -> None:
        c = _make_client(FakeSession())
        status, body = await c.issue_status("not-a-tg-id")
        assert status == 400
        assert "invalid" in body["message"]

    async def test_accepts_plain_numeric_user_id(self) -> None:
        s = FakeSession()
        s.on(
            "GET",
            "/api/users/by-telegram-id/100",
            200,
            json.dumps({"response": {"uuid": "u1", "status": "ACTIVE"}}),
        )
        c = _make_client(s)
        status, _ = await c.issue_status("100")
        assert status == 200

    async def test_non_active_status_maps_to_issued(self) -> None:
        s = FakeSession()
        s.on(
            "GET",
            "/api/users/by-telegram-id/42",
            200,
            json.dumps({"response": {"uuid": "u1", "status": "DISABLED"}}),
        )
        c = _make_client(s)
        _, body = await c.issue_status("tg_42")
        assert body["status"] == "issued"

    async def test_response_without_uuid_is_treated_as_not_found(self) -> None:
        s = FakeSession()
        s.on(
            "GET",
            "/api/users/by-telegram-id/42",
            200,
            json.dumps({"response": {"status": "ACTIVE"}}),
        )
        c = _make_client(s)
        status, body = await c.issue_status("tg_42")
        assert status == 404
        assert "not found" in body["message"]

    async def test_accepts_bare_list_response(self) -> None:
        s = FakeSession()
        s.on(
            "GET",
            "/api/users/by-telegram-id/42",
            200,
            json.dumps([{"uuid": "uuid-1", "status": "ACTIVE"}]),
        )
        c = _make_client(s)
        status, body = await c.issue_status("tg_42")
        assert status == 200
        assert body["subscriptionId"] == "uuid-1"


class TestIssueLink:
    async def test_creates_new_user_when_missing(self) -> None:
        s = FakeSession()
        s.on("GET", "/api/users/by-telegram-id/42", 404, "{}")
        s.on(
            "POST",
            "/api/users",
            201,
            json.dumps(
                {
                    "response": {
                        "uuid": "new-uuid",
                        "subscriptionUrl": "https://panel.example.com/sub/new-uuid",
                    }
                }
            ),
        )
        c = _make_client(s, squads=["squad-1", "squad-2"])

        status, body = await c.issue_link("tg_42", "name", "src", None, "idem-1")

        assert status == 200
        assert body["subscription"]["id"] == "new-uuid"
        assert body["subscription"]["userId"] == "tg_42"
        assert body["url"] == "https://panel.example.com/sub/new-uuid"
        assert body["profileId"] == "user-tg-42"
        assert body["days"] == 30

        post = s.calls_for("POST", "/api/users")[0]
        assert post.json is not None
        assert post.json["telegramId"] == 42
        assert post.json["status"] == "ACTIVE"
        assert post.json["activeInternalSquads"] == ["squad-1", "squad-2"]
        assert post.json["trafficLimitStrategy"] == "NO_RESET"
        assert post.json["username"] == "tg42"
        # expireAt is ~30 days in the future in ISO UTC
        exp = datetime.fromisoformat(post.json["expireAt"].replace("Z", "+00:00"))
        delta_days = (exp - datetime.now(UTC)).days
        assert 28 <= delta_days <= 31

    async def test_creates_user_rejects_when_no_squads(self) -> None:
        s = FakeSession()
        s.on("GET", "/api/users/by-telegram-id/42", 404, "{}")
        c = _make_client(s, squads=[])

        status, body = await c.issue_link("tg_42", "n", "s", None, None)

        assert status == 503
        assert "REMNAWAVE_INTERNAL_SQUAD_UUIDS" in body["message"]
        # Make sure we did NOT POST to /api/users.
        assert not s.calls_for("POST", "/api/users")

    async def test_returns_existing_user_as_200(self) -> None:
        s = FakeSession()
        s.on(
            "GET",
            "/api/users/by-telegram-id/42",
            200,
            json.dumps(
                {
                    "response": {
                        "uuid": "uuid-existing",
                        "status": "ACTIVE",
                        "subscriptionUrl": "https://panel.example.com/sub/uuid-existing",
                    }
                }
            ),
        )
        c = _make_client(s, squads=["any"])

        status, body = await c.issue_link("tg_42", "n", "s", None, None)

        assert status == 200
        assert body["subscription"]["id"] == "uuid-existing"
        assert body["url"] == "https://panel.example.com/sub/uuid-existing"
        # Must NOT create a second user.
        assert not s.calls_for("POST", "/api/users")

    async def test_invalid_user_id_is_400(self) -> None:
        c = _make_client(FakeSession())
        status, _ = await c.issue_link("bogus", "n", "s", None, None)
        assert status == 400

    async def test_non_2xx_create_surfaces_status_and_body(self) -> None:
        s = FakeSession()
        s.on("GET", "/api/users/by-telegram-id/42", 404, "{}")
        s.on("POST", "/api/users", 422, json.dumps({"message": "username taken"}))
        c = _make_client(s, squads=["squad-1"])

        status, body = await c.issue_link("tg_42", "n", "s", None, None)

        assert status == 422
        assert body["message"] == "username taken"


class TestLifecycleRenew:
    async def test_renew_adds_days_to_future_expiry(self) -> None:
        future = datetime.now(UTC) + timedelta(days=10)
        s = FakeSession()
        s.on(
            "GET",
            "/api/users/by-telegram-id/42",
            200,
            json.dumps(
                {
                    "response": {
                        "uuid": "uuid-1",
                        "expireAt": future.isoformat().replace("+00:00", "Z"),
                    }
                }
            ),
        )
        s.on("PATCH", "/api/users", 200, json.dumps({"response": {"ok": True}}))
        c = _make_client(s)

        status, body = await c.lifecycle_renew("tg_42", 7)

        assert status == 200
        assert body == {"ok": True}
        patch_call = s.calls_for("PATCH", "/api/users")[0]
        assert patch_call.json["uuid"] == "uuid-1"
        new_exp = datetime.fromisoformat(patch_call.json["expireAt"].replace("Z", "+00:00"))
        diff = (new_exp - future).total_seconds()
        assert abs(diff - 7 * 86400) < 60  # +7 days ± 1 minute

    async def test_renew_from_past_uses_now_as_baseline(self) -> None:
        past = datetime.now(UTC) - timedelta(days=20)
        s = FakeSession()
        s.on(
            "GET",
            "/api/users/by-telegram-id/42",
            200,
            json.dumps(
                {
                    "response": {
                        "uuid": "uuid-1",
                        "expireAt": past.isoformat().replace("+00:00", "Z"),
                    }
                }
            ),
        )
        s.on("PATCH", "/api/users", 200, "{}")
        c = _make_client(s)

        await c.lifecycle_renew("tg_42", 30)

        patch_call = s.calls_for("PATCH", "/api/users")[0]
        new_exp = datetime.fromisoformat(patch_call.json["expireAt"].replace("Z", "+00:00"))
        expected = datetime.now(UTC) + timedelta(days=30)
        assert abs((new_exp - expected).total_seconds()) < 120

    async def test_renew_404_when_user_missing(self) -> None:
        s = FakeSession()
        s.on("GET", "/api/users/by-telegram-id/42", 404, "{}")
        c = _make_client(s)

        status, _ = await c.lifecycle_renew("tg_42", 7)

        assert status == 404

    async def test_renew_invalid_user_id(self) -> None:
        status, _ = await _make_client(FakeSession()).lifecycle_renew("oops", 7)
        assert status == 400

    async def test_renew_propagates_patch_failure(self) -> None:
        s = FakeSession()
        s.on(
            "GET",
            "/api/users/by-telegram-id/42",
            200,
            json.dumps({"response": {"uuid": "u", "expireAt": None}}),
        )
        s.on("PATCH", "/api/users", 500, json.dumps({"message": "boom"}))
        c = _make_client(s)

        status, body = await c.lifecycle_renew("tg_42", 7)

        assert status == 500
        assert body["message"] == "boom"


class TestGetSubscription:
    async def test_returns_normalized_traffic(self) -> None:
        s = FakeSession()
        s.on(
            "GET",
            "/api/users/uuid-1",
            200,
            json.dumps(
                {
                    "response": {
                        "expireAt": "2030-01-01T00:00:00Z",
                        "trafficLimitBytes": 1024 * 1024 * 1024,
                        "userTraffic": {"usedTrafficBytes": 500 * 1024 * 1024},
                    }
                }
            ),
        )
        c = _make_client(s)

        status, body = await c.get_subscription("uuid-1")

        assert status == 200
        assert body["id"] == "uuid-1"
        assert body["expiresAt"] == "2030-01-01T00:00:00Z"
        assert body["trafficLimitBytes"] == 1024 * 1024 * 1024
        assert body["usedTrafficBytes"] == 500 * 1024 * 1024

    async def test_empty_id_returns_400(self) -> None:
        status, _ = await _make_client(FakeSession()).get_subscription("  ")
        assert status == 400

    async def test_not_found_passes_through(self) -> None:
        s = FakeSession()
        s.on("GET", "/api/users/missing", 404, json.dumps({"message": "nope"}))
        c = _make_client(s)

        status, body = await c.get_subscription("missing")

        assert status == 404
        assert body["message"] == "nope"


class TestGetDeliveryLinks:
    async def test_returns_subscription_url(self) -> None:
        s = FakeSession()
        s.on(
            "GET",
            "/api/users/by-telegram-id/42",
            200,
            json.dumps(
                {
                    "response": {
                        "uuid": "u",
                        "subscriptionUrl": "https://panel.example.com/sub/token-xyz",
                    }
                }
            ),
        )
        c = _make_client(s)

        status, body = await c.get_delivery_links("user-tg-42")

        assert status == 200
        assert body["links"]["subscription"] == "https://panel.example.com/sub/token-xyz"

    async def test_invalid_profile_id(self) -> None:
        c = _make_client(FakeSession())
        status, _ = await c.get_delivery_links("wrong-format")
        assert status == 400

    async def test_missing_user_returns_404_with_empty_links(self) -> None:
        s = FakeSession()
        s.on("GET", "/api/users/by-telegram-id/42", 404, "{}")
        c = _make_client(s)

        status, body = await c.get_delivery_links("user-tg-42")

        assert status == 404
        assert body == {"links": {}}

    async def test_user_without_subscription_url(self) -> None:
        s = FakeSession()
        s.on(
            "GET",
            "/api/users/by-telegram-id/42",
            200,
            json.dumps({"response": {"uuid": "u"}}),
        )
        c = _make_client(s)
        status, body = await c.get_delivery_links("user-tg-42")
        assert status == 404
        assert body == {"links": {}}


class TestHealth:
    async def test_health_passes_through(self) -> None:
        s = FakeSession()
        s.on("GET", "/api/system/health", 200, json.dumps({"ok": True}))
        c = _make_client(s)

        status, body = await c.get_health()

        assert status == 200
        assert body == {"ok": True}


class TestRequestAuthHeaders:
    async def test_bearer_token_is_forwarded_on_every_request(self) -> None:
        s = FakeSession()
        s.on("GET", "/api/system/health", 200, "{}")
        fake_token = "fake-test-token"
        c = _make_client(s, api_token=fake_token)

        await c.get_health()

        call = s.last_call()
        assert call.headers is not None
        assert call.headers["Authorization"] == f"Bearer {fake_token}"


class TestProtocolConformance:
    def test_remnawave_client_matches_vpn_backend_protocol(self) -> None:
        # If the static methods ever drift, `isinstance` with a runtime Protocol
        # cannot check without `@runtime_checkable`, so instead we just exercise
        # the full attribute surface to make sure each entry in VPNBackend exists.
        c = _make_client(FakeSession())
        for attr in (
            "issue_status",
            "issue_link",
            "lifecycle_renew",
            "get_subscription",
            "get_delivery_links",
            "get_health",
        ):
            assert callable(getattr(c, attr)), f"{attr} missing from RemnawaveApiClient"
        # Also exercise the type to make sure assignments won't raise.
        backend: VPNBackend = c  # noqa: F841


class TestNetworkErrorsAreWrapped:
    async def test_client_error_becomes_apierror(self, monkeypatch: pytest.MonkeyPatch) -> None:
        import aiohttp

        class BoomSession(FakeSession):
            def request(self, *a, **kw):  # type: ignore[override]
                raise aiohttp.ClientError("down")

        c = _make_client(BoomSession())

        with pytest.raises(ApiError):
            await c.get_health()
