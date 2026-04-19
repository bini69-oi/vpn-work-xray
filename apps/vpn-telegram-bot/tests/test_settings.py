"""Settings tests for Remnawave integration."""
from __future__ import annotations

import pytest

from vpn_bot.config.settings import Settings


def _build(monkeypatch: pytest.MonkeyPatch, **env: str) -> Settings:
    """Apply env and construct Settings with `.env` discovery disabled.

    Using env vars (instead of kwargs) both exercises the production code path
    and avoids pydantic alias ↔ field-name quirks across pydantic-settings
    versions.
    """
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    return Settings(_env_file=None)  # type: ignore[call-arg]


class TestVpnBackendNormalized:
    @pytest.mark.parametrize(
        "raw,expected",
        [
            ("", "productd"),
            ("productd", "productd"),
            ("PRODUCTD", "productd"),
            (" remnawave ", "remnawave"),
            ("Remnawave", "remnawave"),
            ("garbage", "productd"),
        ],
    )
    def test_cases(self, monkeypatch: pytest.MonkeyPatch, raw: str, expected: str) -> None:
        assert _build(monkeypatch, VPN_BACKEND=raw).vpn_backend_normalized() == expected


class TestApiConfigured:
    def test_productd_requires_token(self, monkeypatch: pytest.MonkeyPatch) -> None:
        assert _build(monkeypatch).api_configured() is False
        assert _build(monkeypatch, VPN_API_TOKEN="x").api_configured() is True

    def test_remnawave_requires_panel_and_token(self, monkeypatch: pytest.MonkeyPatch) -> None:
        s = _build(monkeypatch, VPN_BACKEND="remnawave")
        assert s.api_configured() is False

        s = _build(monkeypatch, VPN_BACKEND="remnawave", REMNAWAVE_API_TOKEN="t")
        assert s.api_configured() is False

        s = _build(
            monkeypatch,
            VPN_BACKEND="remnawave",
            REMNAWAVE_API_TOKEN="t",
            REMNAWAVE_PANEL_URL="https://p.example",
        )
        assert s.api_configured() is True


class TestSquadParsing:
    def test_empty_is_empty_list(self, monkeypatch: pytest.MonkeyPatch) -> None:
        assert _build(monkeypatch).remnawave_internal_squad_uuids() == []

    def test_csv_splitting(self, monkeypatch: pytest.MonkeyPatch) -> None:
        s = _build(monkeypatch, REMNAWAVE_INTERNAL_SQUAD_UUIDS="a,b , ,c")
        assert s.remnawave_internal_squad_uuids() == ["a", "b", "c"]

    def test_singular_alias(self, monkeypatch: pytest.MonkeyPatch) -> None:
        s = _build(monkeypatch, REMNAWAVE_INTERNAL_SQUAD_UUID="only-one")
        assert s.remnawave_internal_squad_uuids() == ["only-one"]


class TestAliasPanelUrl:
    def test_primary_alias(self, monkeypatch: pytest.MonkeyPatch) -> None:
        assert _build(monkeypatch, REMNAWAVE_PANEL_URL="https://x").remnawave_panel_url == "https://x"

    def test_secondary_alias(self, monkeypatch: pytest.MonkeyPatch) -> None:
        assert _build(monkeypatch, REMNAWAVE_BASE_URL="https://y").remnawave_panel_url == "https://y"
