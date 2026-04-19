from __future__ import annotations

import html
from datetime import UTC, datetime
from typing import Any

PRICING: dict[int, dict[str, int]] = {
    1: {"months": 1, "price": 90, "discount": 0},
    2: {"months": 2, "price": 170, "discount": 6},
    3: {"months": 3, "price": 240, "discount": 11},
    6: {"months": 6, "price": 430, "discount": 20},
    12: {"months": 12, "price": 750, "discount": 31},
}


def progress_bar(value: int, max_val: int = 100, length: int = 10) -> str:
    if max_val <= 0:
        return "░" * length
    filled = int(length * max(0, min(max_val, value)) / max_val)
    return "▓" * filled + "░" * (length - filled)


def format_rub(kopecks: int) -> str:
    return f"{kopecks // 100}"


def format_bytes(n: int | float) -> str:
    try:
        v = float(n)
    except Exception:
        return "0 B"
    if v <= 0:
        return "0 B"
    units = ["B", "KB", "MB", "GB", "TB"]
    idx = 0
    while v >= 1024 and idx < len(units) - 1:
        v /= 1024.0
        idx += 1
    if idx == 0:
        return f"{int(v)} {units[idx]}"
    return f"{v:.1f} {units[idx]}"


def parse_iso_dt(raw: str | None) -> datetime | None:
    if not raw:
        return None
    s = str(raw).strip()
    if not s:
        return None
    if s.endswith("Z"):
        s = s[:-1] + "+00:00"
    try:
        return datetime.fromisoformat(s)
    except Exception:
        return None


def format_date_ru(iso_string: str | None) -> str:
    dt = parse_iso_dt(iso_string)
    if not dt:
        return "—"
    return dt.astimezone(UTC).strftime("%d.%m.%Y")


def days_left(iso_string: str | None) -> int:
    dt = parse_iso_dt(iso_string)
    if not dt:
        return 0
    now = datetime.now(tz=UTC)
    delta = dt.astimezone(UTC) - now
    if delta.total_seconds() <= 0:
        return 0
    return int((delta.total_seconds() + 86399) // 86400)


def escape_html_json(data: dict[str, Any]) -> str:
    raw = __import__("json").dumps(data, ensure_ascii=False, indent=2)
    return html.escape(raw[:4000] + ("…" if len(raw) > 4000 else ""))


def extract_subscription(payload: dict[str, Any]) -> dict[str, Any]:
    sub = payload.get("subscription")
    if isinstance(sub, dict):
        return sub
    return {}
