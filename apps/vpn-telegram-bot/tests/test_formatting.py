"""Unit tests for pure formatting helpers."""
from __future__ import annotations

from datetime import UTC, datetime, timedelta

from vpn_bot.utils.formatting import (
    PRICING,
    days_left,
    extract_subscription,
    format_bytes,
    format_date_ru,
    format_rub,
    parse_iso_dt,
    progress_bar,
)


class TestPricing:
    def test_all_plans_present(self) -> None:
        assert set(PRICING) == {1, 2, 3, 6, 12}
        for m, p in PRICING.items():
            assert p["months"] == m
            assert p["price"] > 0


class TestProgressBar:
    def test_zero_max_returns_empty(self) -> None:
        assert progress_bar(5, max_val=0) == "░" * 10

    def test_half_bar(self) -> None:
        b = progress_bar(50, max_val=100, length=10)
        assert b.count("▓") == 5
        assert b.count("░") == 5

    def test_overfilled_clamped(self) -> None:
        assert progress_bar(9999, max_val=100, length=5) == "▓" * 5


class TestFormatRub:
    def test_basic(self) -> None:
        assert format_rub(12345) == "123"


class TestFormatBytes:
    def test_zero(self) -> None:
        assert format_bytes(0) == "0 B"

    def test_negative_treated_as_zero(self) -> None:
        assert format_bytes(-1) == "0 B"

    def test_kb_mb_gb_tb(self) -> None:
        assert format_bytes(2048).endswith(" KB")
        assert format_bytes(2 * 1024 * 1024).endswith(" MB")
        assert format_bytes(5 * 1024 ** 3).endswith(" GB")
        assert format_bytes(3 * 1024 ** 4).endswith(" TB")

    def test_invalid_returns_zero(self) -> None:
        assert format_bytes("x") == "0 B"  # type: ignore[arg-type]


class TestDateParsing:
    def test_parse_iso_z(self) -> None:
        dt = parse_iso_dt("2030-01-02T03:04:05Z")
        assert dt and dt.year == 2030

    def test_parse_none_and_empty(self) -> None:
        assert parse_iso_dt(None) is None
        assert parse_iso_dt("") is None
        assert parse_iso_dt("   ") is None

    def test_parse_invalid(self) -> None:
        assert parse_iso_dt("nope") is None

    def test_format_date_ru_empty(self) -> None:
        assert format_date_ru(None) == "—"
        assert format_date_ru("not-a-date") == "—"

    def test_format_date_ru_ok(self) -> None:
        assert format_date_ru("2030-05-15T10:00:00Z") == "15.05.2030"


class TestDaysLeft:
    def test_none_returns_zero(self) -> None:
        assert days_left(None) == 0

    def test_past_returns_zero(self) -> None:
        past = (datetime.now(UTC) - timedelta(days=2)).isoformat().replace("+00:00", "Z")
        assert days_left(past) == 0

    def test_future_counted(self) -> None:
        fut = (datetime.now(UTC) + timedelta(days=3, hours=1)).isoformat().replace("+00:00", "Z")
        assert days_left(fut) >= 3


class TestExtractSubscription:
    def test_missing_returns_empty_dict(self) -> None:
        assert extract_subscription({}) == {}

    def test_non_dict_is_empty(self) -> None:
        assert extract_subscription({"subscription": "oops"}) == {}

    def test_valid(self) -> None:
        assert extract_subscription({"subscription": {"id": "x"}}) == {"id": "x"}
