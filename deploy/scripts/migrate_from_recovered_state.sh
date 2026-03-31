#!/usr/bin/env bash
set -euo pipefail

# Run on target server as root.
# Uses files previously uploaded to /root/migrate.

NEW_IP="${1:-198.13.186.190}"

export DEBIAN_FRONTEND=noninteractive
apt-get update -y >/dev/null
apt-get install -y ca-certificates curl tar python3 sqlite3 caddy >/dev/null

useradd -r -s /usr/sbin/nologin vpn-product 2>/dev/null || true
mkdir -p /etc/vpn-product /var/lib/vpn-product /var/lib/vpn-product/assets /var/log/vpn-product /etc/x-ui /usr/local/bin /etc/systemd/system
systemctl stop vpn-productd 2>/dev/null || true

install -m 0755 /root/migrate/vpn-productd /usr/local/bin/vpn-productd
install -m 0755 /root/migrate/vpn-xui-runtime-sync.sh /usr/local/bin/vpn-xui-runtime-sync.sh
install -m 0755 /root/migrate/sync-xui-client-limits.py /usr/local/bin/sync-xui-client-limits.py
cp -a /root/migrate/product.db /var/lib/vpn-product/product.db
cp -a /root/migrate/assets/. /var/lib/vpn-product/assets/ || true

rm -rf /usr/local/x-ui
cp -a /root/migrate/x-ui /usr/local/x-ui
chmod 0755 /usr/local/x-ui/x-ui /usr/local/x-ui/bin/xray-linux-amd64

cp -a /root/migrate/vpn-productd.service /etc/systemd/system/vpn-productd.service
cp -a /root/migrate/vpn-xui-runtime-sync.service /etc/systemd/system/vpn-xui-runtime-sync.service
cp -a /root/migrate/vpn-xui-runtime-sync.timer /etc/systemd/system/vpn-xui-runtime-sync.timer
cp -a /root/migrate/vpn-xui-client-limits-sync.service /etc/systemd/system/vpn-xui-client-limits-sync.service
cp -a /root/migrate/vpn-xui-client-limits-sync.timer /etc/systemd/system/vpn-xui-client-limits-sync.timer
cp -a /usr/local/x-ui/x-ui.service.debian /etc/systemd/system/x-ui.service

API_TOKEN="$(openssl rand -base64 32 | tr -d '\n')"
ADMIN_TOKEN="$(openssl rand -hex 16)"
cat > /etc/vpn-product/vpn-productd.env <<EOF
VPN_PRODUCT_API_TOKEN=${API_TOKEN}
VPN_PRODUCT_ADMIN_TOKEN=${ADMIN_TOKEN}
VPN_PRODUCT_ADMIN_ALLOWLIST=127.0.0.1,::1
VPN_PRODUCT_LISTEN=127.0.0.1:8080
VPN_PRODUCT_DATA_DIR=/var/lib/vpn-product
VPN_PRODUCT_LOG_FILE=/var/log/vpn-product/vpn-productd.log
VPN_PRODUCT_3XUI_DB_PATH=/etc/x-ui/x-ui.db
VPN_PRODUCT_3XUI_INBOUND_PORT=8443
EOF

cat > /etc/caddy/Caddyfile <<EOF
${NEW_IP//./-}.sslip.io {
    encode zstd gzip
    reverse_proxy 127.0.0.1:8080
}

xui-${NEW_IP//./-}.sslip.io {
    encode zstd gzip
    reverse_proxy 127.0.0.1:13554
}
EOF

chown -R vpn-product:vpn-product /var/lib/vpn-product /var/log/vpn-product || true
chgrp -R vpn-product /etc/x-ui || true
chmod 775 /etc/x-ui || true

systemctl daemon-reload
systemctl enable --now x-ui >/dev/null 2>&1 || true
sleep 2

python3 - "$NEW_IP" <<'PY'
import json
import sqlite3
import time
import sys

new_ip = sys.argv[1]

cfg = json.load(open("/usr/local/x-ui/bin/config.json", "r", encoding="utf-8"))
vless = None
for ib in cfg.get("inbounds", []):
    if ib.get("protocol") == "vless" and int(ib.get("port", 0)) == 8443:
        vless = ib
        break

if vless:
    conn = sqlite3.connect("/etc/x-ui/x-ui.db")
    cur = conn.cursor()
    row = cur.execute(
        "SELECT id FROM inbounds WHERE protocol='vless' AND port=8443 ORDER BY id DESC LIMIT 1"
    ).fetchone()
    if row:
        inbound_id = row[0]
        cur.execute(
            "UPDATE inbounds SET enable=1, remark=?, listen=?, settings=?, stream_settings=?, tag=?, sniffing=?, total=0, expiry_time=0 WHERE id=?",
            (
                "VPN",
                vless.get("listen", "0.0.0.0"),
                json.dumps(vless.get("settings", {}), separators=(",", ":")),
                json.dumps(vless.get("streamSettings", {}), separators=(",", ":")),
                vless.get("tag", "inbound-8443"),
                json.dumps(vless.get("sniffing", {}), separators=(",", ":")),
                inbound_id,
            ),
        )
    else:
        cur.execute(
            "INSERT INTO inbounds(user_id,up,down,total,all_time,remark,enable,expiry_time,traffic_reset,last_traffic_reset_time,listen,port,protocol,settings,stream_settings,tag,sniffing) VALUES(0,0,0,0,0,?,1,0,'never',0,?,8443,'vless',?,?,?,?)",
            (
                "VPN",
                vless.get("listen", "0.0.0.0"),
                json.dumps(vless.get("settings", {}), separators=(",", ":")),
                json.dumps(vless.get("streamSettings", {}), separators=(",", ":")),
                vless.get("tag", "inbound-8443"),
                json.dumps(vless.get("sniffing", {}), separators=(",", ":")),
            ),
        )
        inbound_id = cur.lastrowid

    clients = vless.get("settings", {}).get("clients", [])
    exp_ms = int((time.time() + 30 * 24 * 3600) * 1000)
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
                "INSERT INTO client_traffics(inbound_id,enable,email,up,down,all_time,expiry_time,total,reset,last_online) VALUES(?,1,?,0,0,0,?, ?,0,0)",
                (inbound_id, email, exp_ms, 1024**4),
            )
    conn.commit()
    conn.close()

pc = sqlite3.connect("/var/lib/vpn-product/product.db")
cur = pc.cursor()
for pid, pj in cur.execute("SELECT id, profile_json FROM profiles").fetchall():
    try:
        d = json.loads(pj)
    except Exception:
        continue
    changed = False
    for ep in d.get("endpoints", []):
        if ep.get("address") != new_ip:
            ep["address"] = new_ip
            changed = True
    if changed:
        cur.execute(
            "UPDATE profiles SET profile_json=?, updated_at=datetime('now') WHERE id=?",
            (json.dumps(d, separators=(",", ":")), pid),
        )
pc.commit()
pc.close()
PY

chmod 664 /etc/x-ui/x-ui.db || true

systemctl enable --now vpn-productd caddy vpn-xui-runtime-sync.timer vpn-xui-client-limits-sync.timer >/dev/null 2>&1 || true
systemctl restart x-ui vpn-productd caddy
systemctl start vpn-xui-runtime-sync.service vpn-xui-client-limits-sync.service || true

echo "API_TOKEN=${API_TOKEN}"
echo "ADMIN_TOKEN=${ADMIN_TOKEN}"
systemctl is-active x-ui vpn-productd caddy
