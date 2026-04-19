"""Tests for `format_health_report` — the admin health-panel renderer."""
from __future__ import annotations

from vpn_bot.services.monitoring_service import format_health_report


class TestFormatHealthReport:
    def test_default_title(self) -> None:
        out = format_health_report(200, {"ok": True})
        assert "Remnawave Panel" in out
        assert "true" in out.lower()

    def test_custom_title(self) -> None:
        out = format_health_report(200, {"ok": True}, title="📊 <b>Custom</b>")
        assert "Custom" in out
        assert "Remnawave Panel" not in out

    def test_non_200_includes_http_code(self) -> None:
        out = format_health_report(503, {"error": "down"})
        assert "503" in out
        assert "down" in out

    def test_truncates_huge_payloads(self) -> None:
        big = {"x": "a" * 100_000}
        out = format_health_report(200, big)
        assert len(out) < 5000  # well under Telegram's 4096 msg limit once HTML-wrapped
