#!/usr/bin/env python3
import json
import sqlite3
from datetime import datetime, timedelta, timezone

PDB = "/var/lib/vpn-product/product.db"
XDB = "/etc/x-ui/x-ui.db"
INBOUND_PORT = 8443
TB_BYTES = 1024**4
TB_GB = 1024


def now_utc_iso() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def main() -> None:
    now = datetime.now(timezone.utc)
    expires = now + timedelta(days=30)
    expires_iso = expires.isoformat().replace("+00:00", "Z")
    expires_ms = int(expires.timestamp() * 1000)
    updated_at = now_utc_iso()

    pc = sqlite3.connect(PDB)
    pcur = pc.cursor()
    subs = pcur.execute(
        "SELECT id, user_id, profile_ids_json FROM subscriptions WHERE revoked = 0"
    ).fetchall()
    users = sorted({(row[1] or "").strip() for row in subs if (row[1] or "").strip()})
    user_client = {}

    for sid, uid, profile_ids_json in subs:
        uid = (uid or "").strip()
        pcur.execute(
            "UPDATE subscriptions SET expires_at = ?, status = 'active', updated_at = ? WHERE id = ?",
            (expires_iso, updated_at, sid),
        )
        try:
            profile_ids = json.loads(profile_ids_json or "[]")
        except Exception:
            profile_ids = []
        for pid in profile_ids:
            prow = pcur.execute(
                "SELECT profile_json FROM profiles WHERE id = ?", (pid,)
            ).fetchone()
            if not prow:
                continue
            pdata = json.loads(prow[0] or "{}")
            if uid:
                endpoints = pdata.get("endpoints") or []
                if endpoints:
                    ep0 = endpoints[0] or {}
                    user_client[uid] = {
                        "uuid": str(ep0.get("uuid") or ""),
                        "flow": str(ep0.get("flow") or "xtls-rprx-vision"),
                    }
            pdata["trafficLimitGb"] = TB_GB
            pdata["subscriptionExpiresAt"] = expires_iso
            pdata["blocked"] = False
            pcur.execute(
                "UPDATE profiles SET profile_json = ?, enabled = 1, updated_at = ? WHERE id = ?",
                (json.dumps(pdata, separators=(",", ":")), updated_at, pid),
            )
            qrow = pcur.execute(
                "SELECT profile_id FROM profile_quota WHERE profile_id = ?", (pid,)
            ).fetchone()
            if qrow:
                pcur.execute(
                    "UPDATE profile_quota SET traffic_limit_gb = ?, expires_at = ?, blocked = 0, updated_at = ? WHERE profile_id = ?",
                    (TB_GB, expires_iso, updated_at, pid),
                )
            else:
                pcur.execute(
                    "INSERT INTO profile_quota(profile_id, limit_mb, used_upload_bytes, used_download_bytes, expires_at, traffic_limit_gb, traffic_used_bytes, blocked, updated_at) VALUES(?,0,0,0,?,?,0,0,?)",
                    (pid, expires_iso, TB_GB, updated_at),
                )
    pc.commit()
    pc.close()

    xc = sqlite3.connect(XDB)
    xcur = xc.cursor()
    inbound = xcur.execute(
        "SELECT id, settings FROM inbounds WHERE protocol = 'vless' AND port = ? ORDER BY id DESC LIMIT 1",
        (INBOUND_PORT,),
    ).fetchone()
    if inbound:
        inbound_id, settings_raw = inbound
        settings = json.loads(settings_raw or "{}")
        clients = settings.get("clients", [])
        by_email = {}
        for cl in clients:
            email = str(cl.get("email", "")).strip()
            if email:
                by_email[email] = cl

        # Include all known x-ui users too (not only users from active subscriptions).
        xui_users = {
            (row[0] or "").strip()
            for row in xcur.execute(
                "SELECT DISTINCT email FROM client_traffics WHERE inbound_id = ?",
                (inbound_id,),
            ).fetchall()
            if (row[0] or "").strip()
        }
        all_users = sorted(set(users) | set(by_email.keys()) | xui_users)

        for email in all_users:
            cl = by_email.get(email)
            if cl:
                cl["enable"] = True
                cl["totalGB"] = TB_GB
                cl["expiryTime"] = expires_ms
                cl.setdefault("limitIp", 0)
            else:
                spec = user_client.get(email) or {}
                client_uuid = str(spec.get("uuid") or "").strip()
                if client_uuid:
                    clients.append(
                        {
                            "id": client_uuid,
                            "flow": str(spec.get("flow") or "xtls-rprx-vision"),
                            "email": email,
                            "enable": True,
                            "expiryTime": expires_ms,
                            "totalGB": TB_GB,
                            "limitIp": 0,
                        }
                    )

            row = xcur.execute(
                "SELECT id FROM client_traffics WHERE inbound_id = ? AND email = ? ORDER BY id DESC LIMIT 1",
                (inbound_id, email),
            ).fetchone()
            if row:
                xcur.execute(
                    "UPDATE client_traffics SET enable = 1, total = ?, expiry_time = ? WHERE id = ?",
                    (TB_BYTES, expires_ms, row[0]),
                )
            else:
                xcur.execute(
                    "INSERT INTO client_traffics(inbound_id, enable, email, up, down, all_time, expiry_time, total, reset, last_online) VALUES(?,1,?,0,0,0,?,?,0,0)",
                    (inbound_id, email, expires_ms, TB_BYTES),
                )

        settings["clients"] = clients
        xcur.execute(
            "UPDATE inbounds SET settings = ?, total = 0, expiry_time = 0 WHERE id = ?",
            (json.dumps(settings, separators=(",", ":")), inbound_id),
        )

    xc.commit()
    xc.close()

    print("UPDATED_USERS:", ",".join(users))
    print("EXPIRES_AT:", expires_iso)


if __name__ == "__main__":
    main()
