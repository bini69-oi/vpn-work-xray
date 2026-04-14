from __future__ import annotations

import json
from typing import Any


def format_health_report(status: int, data: dict[str, Any]) -> str:
    title = "📊 <b>Состояние vpn-productd</b>"
    if status != 200:
        return f"{title}\n\n⚠️ HTTP <code>{status}</code>\n<pre>{json.dumps(data, ensure_ascii=False, indent=2)[:3500]}</pre>"
    body = json.dumps(data, ensure_ascii=False, indent=2)
    if len(body) > 3800:
        body = body[:3800] + "\n…"
    return f"{title}\n\n<pre>{body}</pre>"
