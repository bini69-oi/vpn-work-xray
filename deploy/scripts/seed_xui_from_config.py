#!/usr/bin/env python3
import json
import sqlite3
import time

RECOVERED_CFG = "/root/migrate/x-ui/bin/config.json"
XUI_DB = "/etc/x-ui/x-ui.db"


def main() -> None:
    cfg = json.load(open(RECOVERED_CFG, "r", encoding="utf-8"))
    vless = None
    for ib in cfg.get("inbounds", []):
        if ib.get("protocol") == "vless" and int(ib.get("port", 0)) == 8443:
            vless = ib
            break
    if not vless:
        raise SystemExit("no vless inbound in recovered config")

    conn = sqlite3.connect(XUI_DB)
    cur = conn.cursor()
    row = cur.execute(
        "SELECT id FROM inbounds WHERE protocol='vless' AND port=8443 ORDER BY id DESC LIMIT 1"
    ).fetchone()
    settings = json.dumps(vless.get("settings", {}), separators=(",", ":"))
    stream = json.dumps(vless.get("streamSettings", {}), separators=(",", ":"))
    sniff = json.dumps(vless.get("sniffing", {}), separators=(",", ":"))

    if row:
        inbound_id = row[0]
        cur.execute(
            "UPDATE inbounds SET remark=?,enable=1,listen=?,settings=?,stream_settings=?,tag=?,sniffing=?,total=0,expiry_time=0 WHERE id=?",
            (
                "VPN",
                vless.get("listen", "0.0.0.0"),
                settings,
                stream,
                vless.get("tag", "inbound-8443"),
                sniff,
                inbound_id,
            ),
        )
    else:
        cur.execute(
            "INSERT INTO inbounds(user_id,up,down,total,all_time,remark,enable,expiry_time,traffic_reset,last_traffic_reset_time,listen,port,protocol,settings,stream_settings,tag,sniffing) VALUES(0,0,0,0,0,?,1,0,'never',0,?,8443,'vless',?,?,?,?)",
            (
                "VPN",
                vless.get("listen", "0.0.0.0"),
                settings,
                stream,
                vless.get("tag", "inbound-8443"),
                sniff,
            ),
        )
        inbound_id = cur.lastrowid

    exp_ms = int((time.time() + 30 * 24 * 3600) * 1000)
    clients = vless.get("settings", {}).get("clients", [])
    for cl in clients:
        email = str(cl.get("email", "")).strip()
        if not email:
            continue
        row = cur.execute(
            "SELECT id FROM client_traffics WHERE inbound_id=? AND email=? ORDER BY id DESC LIMIT 1",
            (inbound_id, email),
        ).fetchone()
        if row:
            cur.execute(
                "UPDATE client_traffics SET enable=1,total=?,expiry_time=? WHERE id=?",
                (1024**4, exp_ms, row[0]),
            )
        else:
            cur.execute(
                "INSERT INTO client_traffics(inbound_id,enable,email,up,down,all_time,expiry_time,total,reset,last_online) VALUES(?,1,?,0,0,0,?,?,0,0)",
                (inbound_id, email, exp_ms, 1024**4),
            )

    conn.commit()
    conn.close()
    print(f"inbound_id={inbound_id} clients={len(clients)}")


if __name__ == "__main__":
    main()
