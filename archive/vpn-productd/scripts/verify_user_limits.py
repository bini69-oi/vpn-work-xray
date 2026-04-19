#!/usr/bin/env python3
import json
import sqlite3

PDB = "/var/lib/vpn-product/product.db"
XDB = "/etc/x-ui/x-ui.db"


def main() -> None:
    pc = sqlite3.connect(PDB)
    pcur = pc.cursor()
    users = list(
        pcur.execute(
            "SELECT user_id, expires_at FROM subscriptions WHERE revoked=0 ORDER BY user_id"
        )
    )
    pc.close()

    print("ACTIVE_SUBSCRIPTIONS")
    for user, exp in users:
        print(user, exp)

    xc = sqlite3.connect(XDB)
    xcur = xc.cursor()
    inbound = xcur.execute(
        "SELECT settings,total,expiry_time FROM inbounds WHERE protocol='vless' AND port=8443 ORDER BY id DESC LIMIT 1"
    ).fetchone()
    settings = json.loads(inbound[0] or "{}")
    print("INBOUND_LIMITS", inbound[1], inbound[2])
    for user, _ in users:
        cl = next((c for c in settings.get("clients", []) if c.get("email") == user), {})
        ct = xcur.execute(
            "SELECT total,expiry_time,enable FROM client_traffics WHERE email=? ORDER BY id DESC LIMIT 1",
            (user,),
        ).fetchone()
        print(
            "USER",
            user,
            "SETTINGS",
            cl.get("totalGB"),
            cl.get("expiryTime"),
            cl.get("enable"),
            "TRAFFIC",
            ct,
        )
    xc.close()


if __name__ == "__main__":
    main()
