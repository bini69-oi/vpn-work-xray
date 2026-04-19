"""Settings tests for the Remnawave-only bot."""
from __future__ import annotations

import pytest

from vpn_bot.config.settings import Settings


def _build(monkeypatch: pytest.MonkeyPatch, **env: str) -> Settings:
    """Apply env and construct Settings with `.env` discovery disabled.

    Using env vars (instead of kwargs) exercises the production code path and
    avoids pydantic alias ↔ field-name quirks across pydantic-settings minor
    versions.
    """
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    return Settings(_env_file=None)  # type: ignore[call-arg]


class TestApiConfigured:
    def test_no_env_is_false(self, monkeypatch: pytest.MonkeyPatch) -> None:
        assert _build(monkeypatch).api_configured() is False

    def test_only_token_is_false(self, monkeypatch: pytest.MonkeyPatch) -> None:
        assert _build(monkeypatch, REMNAWAVE_API_TOKEN="t").api_configured() is False

    def test_only_url_is_false(self, monkeypatch: pytest.MonkeyPatch) -> None:
        assert _build(monkeypatch, REMNAWAVE_PANEL_URL="https://p").api_configured() is False

    def test_both_is_true(self, monkeypatch: pytest.MonkeyPatch) -> None:
        s = _build(
            monkeypatch,
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


class TestAdminIds:
    def test_csv(self, monkeypatch: pytest.MonkeyPatch) -> None:
        assert _build(monkeypatch, ADMIN_IDS="1, 2 , 3").admin_ids() == {1, 2, 3}

    def test_legacy_alias(self, monkeypatch: pytest.MonkeyPatch) -> None:
        assert _build(monkeypatch, ADMIN_TELEGRAM_IDS="42").admin_ids() == {42}

    def test_negative_supergroup_id(self, monkeypatch: pytest.MonkeyPatch) -> None:
        assert _build(monkeypatch, ADMIN_IDS="-100123,7").admin_ids() == {-100123, 7}


class TestAllowedIds:
    def test_empty_is_none(self, monkeypatch: pytest.MonkeyPatch) -> None:
        assert _build(monkeypatch).allowed_ids() is None

    def test_set_returns_ids(self, monkeypatch: pytest.MonkeyPatch) -> None:
        assert _build(monkeypatch, ALLOWED_TELEGRAM_IDS="1,2").allowed_ids() == {1, 2}


class TestReferralBonusDays:
    @pytest.mark.parametrize("raw,want", [("", 15), ("0", 1), ("not-a-num", 15), ("9999", 3650), ("30", 30)])
    def test_clamping(self, monkeypatch: pytest.MonkeyPatch, raw: str, want: int) -> None:
        assert _build(monkeypatch, REFERRAL_BONUS_DAYS=raw).referral_bonus_days == want


class TestPaymentStub:
    @pytest.mark.parametrize(
        "raw,enabled",
        [("0", False), ("", False), ("1", True), ("true", True), ("YES", True), ("off", False)],
    )
    def test_bool_parsing(self, monkeypatch: pytest.MonkeyPatch, raw: str, enabled: bool) -> None:
        assert _build(monkeypatch, PAYMENT_STUB=raw).payment_stub_enabled is enabled

    @pytest.mark.parametrize("raw,result", [("ok", "ok"), ("fail", "fail"), ("garbage", "ok"), ("", "ok")])
    def test_result_is_normalized(self, monkeypatch: pytest.MonkeyPatch, raw: str, result: str) -> None:
        assert _build(monkeypatch, PAYMENT_STUB_RESULT=raw).payment_stub_result == result


class TestDatabaseFile:
    def test_relative_path_anchored_to_bot_root(self, monkeypatch: pytest.MonkeyPatch) -> None:
        s = _build(monkeypatch, DATABASE_PATH="data/x.db")
        assert s.database_file().is_absolute()
        assert s.database_file().name == "x.db"

    def test_absolute_path_kept_as_is(self, monkeypatch: pytest.MonkeyPatch, tmp_path) -> None:
        target = tmp_path / "abs.db"
        s = _build(monkeypatch, DATABASE_PATH=str(target))
        assert s.database_file() == target
