"""Tests for the backend-client factory in `vpn_bot.app`.

The factory chooses between `VPNApiClient` and `RemnawaveApiClient` based on
environment variables. We import `_build_backend_client` from `vpn_bot.app`
and patch the module-level `settings` object with a custom `Settings`
instance so that behaviour is deterministic.
"""
from __future__ import annotations

import pytest

from tests._fake_session import FakeSession
from vpn_bot.config.settings import Settings
from vpn_bot.services.api_client import VPNApiClient
from vpn_bot.services.remnawave_client import RemnawaveApiClient


# All env variables any test in this module writes. We clear them between
# consecutive `patch_settings(...)` invocations within one test so stale state
# doesn't leak.
_MANAGED_ENV = (
    "VPN_BACKEND",
    "VPN_API_TOKEN",
    "VPN_API_URL",
    "REMNAWAVE_PANEL_URL",
    "REMNAWAVE_BASE_URL",
    "REMNAWAVE_API_TOKEN",
    "REMNAWAVE_CADDY_TOKEN",
    "REMNAWAVE_INTERNAL_SQUAD_UUIDS",
    "REMNAWAVE_INTERNAL_SQUAD_UUID",
)


@pytest.fixture
def patch_settings(monkeypatch: pytest.MonkeyPatch):
    """Yield a helper that replaces `vpn_bot.app.settings` with a rebuilt instance.

    Env is applied via monkeypatch (not pydantic kwargs) so the test exercises
    the real production env-parsing path, which behaves identically across
    pydantic-settings 2.x minor versions.
    """
    import vpn_bot.app as app

    def _apply(**env: str) -> None:
        for key in _MANAGED_ENV:
            monkeypatch.delenv(key, raising=False)
        for k, v in env.items():
            monkeypatch.setenv(k, v)
        monkeypatch.setattr(app, "settings", Settings(_env_file=None))  # type: ignore[call-arg]

    return _apply


class TestBuildBackendClient:
    def test_productd_is_default_when_token_present(self, patch_settings) -> None:
        import vpn_bot.app as app

        patch_settings(VPN_API_TOKEN="tok", VPN_API_URL="http://host:1")
        client = app._build_backend_client(FakeSession())  # type: ignore[arg-type]

        assert isinstance(client, VPNApiClient)

    def test_productd_without_token_returns_none(self, patch_settings) -> None:
        import vpn_bot.app as app

        patch_settings()
        assert app._build_backend_client(FakeSession()) is None  # type: ignore[arg-type]

    def test_remnawave_requires_url_and_token(self, patch_settings) -> None:
        import vpn_bot.app as app

        patch_settings(VPN_BACKEND="remnawave", REMNAWAVE_API_TOKEN="t")
        assert app._build_backend_client(FakeSession()) is None  # type: ignore[arg-type]

        patch_settings(VPN_BACKEND="remnawave", REMNAWAVE_PANEL_URL="https://p")
        assert app._build_backend_client(FakeSession()) is None  # type: ignore[arg-type]

    def test_remnawave_builds_client(self, patch_settings) -> None:
        import vpn_bot.app as app

        patch_settings(
            VPN_BACKEND="remnawave",
            REMNAWAVE_API_TOKEN="tok",
            REMNAWAVE_PANEL_URL="https://panel.example",
            REMNAWAVE_INTERNAL_SQUAD_UUIDS="s1",
            REMNAWAVE_CADDY_TOKEN="caddy",
        )
        c = app._build_backend_client(FakeSession())  # type: ignore[arg-type]

        assert isinstance(c, RemnawaveApiClient)
        assert c._root == "https://panel.example/api"
        assert c._squads == ["s1"]
        assert c._headers.get("X-Api-Key") == "caddy"

    def test_remnawave_without_squads_still_builds(self, patch_settings, caplog) -> None:
        import vpn_bot.app as app

        patch_settings(
            VPN_BACKEND="remnawave",
            REMNAWAVE_API_TOKEN="tok",
            REMNAWAVE_PANEL_URL="https://panel.example",
        )
        with caplog.at_level("WARNING", logger="vpn_bot.app"):
            c = app._build_backend_client(FakeSession())  # type: ignore[arg-type]

        assert isinstance(c, RemnawaveApiClient)
        assert any("REMNAWAVE_INTERNAL_SQUAD_UUIDS" in rec.message for rec in caplog.records)
