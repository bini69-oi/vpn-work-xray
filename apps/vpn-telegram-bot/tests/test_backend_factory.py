"""Tests for the Remnawave backend-client factory in `vpn_bot.app`."""
from __future__ import annotations

import pytest

from tests._fake_session import FakeSession
from vpn_bot.config.settings import Settings
from vpn_bot.services.remnawave_client import RemnawaveApiClient

# Variables this module flips between test cases. We clear them between
# `patch_settings(...)` invocations so stale values from one case don't leak
# into another.
_MANAGED_ENV = (
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
    the real production env-parsing path.
    """
    from vpn_bot import app

    def _apply(**env: str) -> None:
        for key in _MANAGED_ENV:
            monkeypatch.delenv(key, raising=False)
        for k, v in env.items():
            monkeypatch.setenv(k, v)
        monkeypatch.setattr(app, "settings", Settings(_env_file=None))  # type: ignore[call-arg]

    return _apply


class TestBuildBackendClient:
    def test_no_token_returns_none(self, patch_settings) -> None:
        from vpn_bot import app

        patch_settings()
        assert app._build_backend_client(FakeSession()) is None  # type: ignore[arg-type]

    def test_only_url_returns_none(self, patch_settings) -> None:
        from vpn_bot import app

        patch_settings(REMNAWAVE_PANEL_URL="https://p")
        assert app._build_backend_client(FakeSession()) is None  # type: ignore[arg-type]

    def test_only_token_returns_none(self, patch_settings) -> None:
        from vpn_bot import app

        patch_settings(REMNAWAVE_API_TOKEN="t")
        assert app._build_backend_client(FakeSession()) is None  # type: ignore[arg-type]

    def test_full_config_builds_client(self, patch_settings) -> None:
        from vpn_bot import app

        patch_settings(
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

    def test_without_squads_still_builds_with_warning(self, patch_settings, caplog) -> None:
        from vpn_bot import app

        patch_settings(
            REMNAWAVE_API_TOKEN="tok",
            REMNAWAVE_PANEL_URL="https://panel.example",
        )
        with caplog.at_level("WARNING", logger="vpn_bot.app"):
            c = app._build_backend_client(FakeSession())  # type: ignore[arg-type]

        assert isinstance(c, RemnawaveApiClient)
        assert any("REMNAWAVE_INTERNAL_SQUAD_UUIDS" in rec.message for rec in caplog.records)

    def test_without_config_logs_hint(self, patch_settings, caplog) -> None:
        from vpn_bot import app

        patch_settings()
        with caplog.at_level("WARNING", logger="vpn_bot.app"):
            assert app._build_backend_client(FakeSession()) is None  # type: ignore[arg-type]
        assert any("REMNAWAVE_PANEL_URL" in rec.message for rec in caplog.records)
