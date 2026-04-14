from __future__ import annotations

import re
from dataclasses import dataclass
from datetime import datetime, timezone


def format_bytes(n: int | float) -> str:
    try:
        v = float(n)
    except Exception:
        return "0 B"
    if v <= 0:
        return "0 B"
    units = ["B", "KB", "MB", "GB", "TB", "PB"]
    idx = 0
    while v >= 1024 and idx < len(units) - 1:
        v /= 1024.0
        idx += 1
    if idx == 0:
        return f"{int(v)} {units[idx]}"
    return f"{v:.1f} {units[idx]}"


def format_date(iso_string: str | None) -> str:
    dt = parse_api_datetime(iso_string)
    if not dt:
        return "—"
    return dt.astimezone(timezone.utc).strftime("%d.%m.%Y")


def days_left(iso_string: str | None) -> int:
    dt = parse_api_datetime(iso_string)
    if not dt:
        return 0
    now = datetime.now(tz=timezone.utc)
    delta = dt.astimezone(timezone.utc) - now
    if delta.total_seconds() <= 0:
        return 0
    return int((delta.total_seconds() + 86399) // 86400)


def progress_bar(used: int, total: int, width: int = 18) -> str:
    if total <= 0:
        return "░" * width + " 0%"
    ratio = max(0.0, min(1.0, float(used) / float(total)))
    filled = int(round(ratio * width))
    bar = "▓" * filled + "░" * (width - filled)
    pct = int(round(ratio * 100))
    return f"{bar} {pct}%"


def parse_api_datetime(raw: str | None) -> datetime | None:
    if not raw:
        return None
    s = str(raw).strip()
    if not s:
        return None
    if s.endswith("Z"):
        s = s[:-1] + "+00:00"
    if "." in s:
        head, tail = s.split(".", 1)
        frac = tail
        tz = ""
        if "+" in tail:
            frac, tz = tail.split("+", 1)
            tz = "+" + tz
        elif "-" in tail[1:]:
            frac, tz = tail.rsplit("-", 1)
            tz = "-" + tz
        frac = re.sub(r"[^0-9]", "", frac)
        frac = (frac + "000000")[:6]
        s = f"{head}.{frac}{tz}"
    try:
        return datetime.fromisoformat(s)
    except Exception:
        return None


def user_id_for_api(telegram_user_id: int) -> str:
    return f"tg_{telegram_user_id}"


def user_profile_id(telegram_user_id: int) -> str:
    return f"user-tg-{telegram_user_id}"


@dataclass(frozen=True)
class RateLimitResult:
    ok: bool
    wait_seconds: int


def check_rate_limit(last_ts: float | None, now_ts: float, window_seconds: int) -> RateLimitResult:
    if window_seconds <= 0:
        return RateLimitResult(ok=True, wait_seconds=0)
    if last_ts is None:
        return RateLimitResult(ok=True, wait_seconds=0)
    delta = now_ts - float(last_ts)
    if delta >= window_seconds:
        return RateLimitResult(ok=True, wait_seconds=0)
    return RateLimitResult(ok=False, wait_seconds=max(1, int(window_seconds - delta)))

