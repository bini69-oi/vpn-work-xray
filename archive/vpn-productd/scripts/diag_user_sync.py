#!/usr/bin/env python3
import json
import sqlite3
import sys

user = sys.argv[1] if len(sys.argv) > 1 else "test_user"

pc = sqlite3.connect("/var/lib/vpn-product/product.db")
cur = pc.cursor()
sub = cur.execute(
    "SELECT id, token, profile_ids_json FROM subscriptions WHERE user_id=? AND revoked=0 ORDER BY created_at DESC LIMIT 1",
    (user,),
).fetchone()
print("SUB", sub[0] if sub else None, sub[1] if sub else None)
if sub:
    pids = json.loads(sub[2] or "[]")
    print("PIDS", pids)
    if pids:
        row = cur.execute(
            "SELECT id, profile_json FROM profiles WHERE id=?",
            (pids[0],),
        ).fetchone()
        if row:
            pj = json.loads(row[1])
            ep = (pj.get("endpoints") or [{}])[0]
            print(
                "PRODUCT_EP",
                ep.get("uuid"),
                ep.get("address"),
                ep.get("port"),
                ep.get("realityPublicKey"),
                ep.get("realityShortId"),
                ep.get("serverName"),
            )
pc.close()

xc = sqlite3.connect("/etc/x-ui/x-ui.db")
cur = xc.cursor()
settings_row = cur.execute(
    "SELECT settings FROM inbounds WHERE protocol='vless' AND port=8443 ORDER BY id DESC LIMIT 1"
).fetchone()
if settings_row:
    clients = json.loads(settings_row[0] or "{}").get("clients", [])
    item = next((c for c in clients if c.get("email") == user), None)
    print("XUI_DB_CLIENT", item)
xc.close()

rc = json.load(open("/usr/local/x-ui/bin/config.json", "r", encoding="utf-8"))
rt = [
    c
    for ib in rc.get("inbounds", [])
    if ib.get("protocol") == "vless" and ib.get("port") == 8443
    for c in ib.get("settings", {}).get("clients", [])
    if c.get("email") == user
]
print("XUI_RT_CLIENT", rt[0] if rt else None)
for ib in rc.get("inbounds", []):
    if ib.get("protocol") == "vless" and ib.get("port") == 8443:
        rs = ib.get("streamSettings", {}).get("realitySettings", {})
        print("REALITY", rs.get("privateKey"), rs.get("shortIds"), rs.get("serverNames"), rs.get("dest"))
