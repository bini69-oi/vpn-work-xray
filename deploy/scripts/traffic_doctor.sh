#!/usr/bin/env bash
set -euo pipefail

# One-command traffic diagnosis for n/a / no traffic cases.
# Usage:
#   sudo bash deploy/scripts/traffic_doctor.sh <user_id>
# Defaults:
#   user_id=test_user

USER_ID="${1:-test_user}"
XUI_DB="${XUI_DB:-/etc/x-ui/x-ui.db}"
RUNTIME_CONFIG="${RUNTIME_CONFIG:-/usr/local/x-ui/bin/config.json}"
ENV_FILE="${ENV_FILE:-/etc/vpn-product/vpn-productd.env}"
XRAY_BIN="${XRAY_BIN:-/usr/local/x-ui/bin/xray-linux-amd64}"
INBOUND_PORT="${INBOUND_PORT:-8443}"
VPN_API="${VPN_API:-http://127.0.0.1:8080}"

ok() { echo "[OK] $*"; }
warn() { echo "[WARN] $*"; }
fail() { echo "[FAIL] $*"; }

echo "=== traffic doctor: ${USER_ID} ==="

systemctl is-active --quiet vpn-productd && ok "vpn-productd active" || fail "vpn-productd inactive"
systemctl is-active --quiet x-ui && ok "x-ui active" || fail "x-ui inactive"
systemctl is-active --quiet caddy && ok "caddy active" || warn "caddy inactive"

if ss -lnt "( sport = :${INBOUND_PORT} )" | awk 'NR>1 {found=1} END{exit(found?0:1)}'; then
  ok "port ${INBOUND_PORT} is listening"
else
  fail "port ${INBOUND_PORT} is not listening"
fi

python3 - <<'PY' "${USER_ID}" "${XUI_DB}" "${RUNTIME_CONFIG}"
import json, sqlite3, sys
user, xdb, runtime_path = sys.argv[1], sys.argv[2], sys.argv[3]

print("--- x-ui db checks ---")
conn = sqlite3.connect(xdb)
cur = conn.cursor()
row = cur.execute(
    "SELECT up,down,total,expiry_time,enable,last_online FROM client_traffics WHERE email=? ORDER BY id DESC LIMIT 1",
    (user,),
).fetchone()
if not row:
    print("[FAIL] client_traffics row not found for user")
else:
    up, down, total, expiry, enable, last_online = row
    print(f"[OK] client_traffics up={up} down={down} total={total} expiry={expiry} enable={enable} last_online={last_online}")

inbound = cur.execute(
    "SELECT settings FROM inbounds WHERE protocol='vless' AND port=8443 ORDER BY id DESC LIMIT 1"
).fetchone()
if not inbound:
    print("[FAIL] vless:8443 inbound not found in x-ui db")
else:
    settings = json.loads(inbound[0] or "{}")
    clients = settings.get("clients", [])
    item = next((c for c in clients if str(c.get("email", "")).strip() == user), None)
    if not item:
        print("[FAIL] user client is absent in inbounds.settings.clients")
    else:
        print(f"[OK] settings.clients item id={item.get('id')} flow={item.get('flow')} limitIp={item.get('limitIp')} enable={item.get('enable')}")

print("--- runtime config checks ---")
cfg = json.load(open(runtime_path, "r", encoding="utf-8"))
vless = next((ib for ib in cfg.get("inbounds", []) if ib.get("protocol") == "vless" and ib.get("port") == 8443), None)
if not vless:
    print("[FAIL] runtime inbound vless:8443 not found")
else:
    rt_clients = vless.get("settings", {}).get("clients", [])
    rt_item = next((c for c in rt_clients if str(c.get("email", "")).strip() == user), None)
    if not rt_item:
        print("[FAIL] runtime client for user not found")
    else:
        print(f"[OK] runtime client id={rt_item.get('id')} flow={rt_item.get('flow')}")
    rs = vless.get("streamSettings", {}).get("realitySettings", {})
    short_ids = rs.get("shortIds") or []
    server_names = rs.get("serverNames") or []
    if rs.get("privateKey"):
        print("[OK] reality privateKey exists")
    else:
        print("[FAIL] reality privateKey missing")
    print(f"[OK] reality shortIds={len(short_ids)} serverNames={len(server_names)} dest={rs.get('dest')}")
conn.close()
PY

if [[ -f "${XRAY_BIN}" && -f "${RUNTIME_CONFIG}" ]]; then
  PRIV_KEY="$(python3 - <<'PY' "${RUNTIME_CONFIG}"
import json,sys
cfg=json.load(open(sys.argv[1],"r",encoding="utf-8"))
vless=next((ib for ib in cfg.get("inbounds",[]) if ib.get("protocol")=="vless" and ib.get("port")==8443),{})
print((vless.get("streamSettings",{}).get("realitySettings",{}) or {}).get("privateKey",""))
PY
)"
  if [[ -n "${PRIV_KEY}" ]]; then
    PUB="$("${XRAY_BIN}" x25519 -i "${PRIV_KEY}" 2>/dev/null | sed -n 's/^Public key: //p')"
    if [[ -n "${PUB}" ]]; then
      ok "derived reality public key: ${PUB}"
    else
      warn "unable to derive reality public key"
    fi
  else
    fail "reality private key is empty"
  fi
else
  warn "xray binary or runtime config not found for reality key derivation"
fi

if [[ -f "${ENV_FILE}" ]]; then
  ADMIN_TOKEN="$(sed -n 's/^VPN_PRODUCT_ADMIN_TOKEN=//p' "${ENV_FILE}" | tr -d '\r\n')"
  if [[ -n "${ADMIN_TOKEN}" ]]; then
    HTTP_CODE="$(curl -s -o /tmp/vpn_doctor_sub.txt -w '%{http_code}' -H "Authorization: Bearer ${ADMIN_TOKEN}" -H 'Content-Type: application/json' -d "{\"userId\":\"${USER_ID}\",\"source\":\"doctor\"}" "${VPN_API}/admin/issue/link" || true)"
    if [[ "${HTTP_CODE}" == "200" ]]; then
      ok "issue-link API works for user"
    else
      fail "issue-link API failed with HTTP ${HTTP_CODE}"
    fi
  else
    warn "VPN_PRODUCT_ADMIN_TOKEN missing in env file"
  fi
else
  warn "env file not found: ${ENV_FILE}"
fi

echo "=== doctor complete ==="
