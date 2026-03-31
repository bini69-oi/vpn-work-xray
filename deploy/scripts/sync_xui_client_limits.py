#!/usr/bin/env python3
import json
import sqlite3

DB = "/etc/x-ui/x-ui.db"
PORT = 8443
GB = 1024**3
DEFAULT_LIMIT_IP = 3


def total_gb(total_bytes: int) -> int:
    if total_bytes <= 0:
        return 0
    return max(1, total_bytes // GB)


def main() -> None:
    conn = sqlite3.connect(DB)
    cur = conn.cursor()
    row = cur.execute(
        "SELECT id, settings FROM inbounds WHERE protocol='vless' AND port=? ORDER BY id DESC LIMIT 1",
        (PORT,),
    ).fetchone()
    if not row:
        conn.close()
        return

    inbound_id, settings_raw = row
    settings = json.loads(settings_raw or "{}")
    clients = settings.get("clients", [])
    changed = 0

    for cl in clients:
        email = str(cl.get("email", "")).strip()
        if not email:
            continue
        tr = cur.execute(
            "SELECT COALESCE(total,0), COALESCE(expiry_time,0), COALESCE(enable,1) FROM client_traffics WHERE inbound_id=? AND email=? ORDER BY id DESC LIMIT 1",
            (inbound_id, email),
        ).fetchone()
        if not tr:
            continue
        total, expiry_ms, enable = int(tr[0]), int(tr[1]), int(tr[2])
        new_total_gb = total_gb(total)
        before = (cl.get("totalGB"), cl.get("expiryTime"), cl.get("enable"), cl.get("limitIp"))
        cl["totalGB"] = new_total_gb
        cl["expiryTime"] = expiry_ms
        cl["enable"] = bool(enable)
        cl["limitIp"] = DEFAULT_LIMIT_IP
        after = (cl.get("totalGB"), cl.get("expiryTime"), cl.get("enable"), cl.get("limitIp"))
        if before != after:
            changed += 1

    if changed > 0:
        settings["clients"] = clients
        cur.execute(
            "UPDATE inbounds SET settings=?, total=0, expiry_time=0 WHERE id=?",
            (json.dumps(settings, separators=(",", ":")), inbound_id),
        )
        conn.commit()

    conn.close()


if __name__ == "__main__":
    main()
