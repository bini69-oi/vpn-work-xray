#!/usr/bin/env bash
set -euo pipefail

# Sync one enabled VLESS+Reality inbound from 3x-ui DB into vpn-product profile,
# then optionally bind an existing subscription token to this profile.
#
# Required on server:
#   - /etc/x-ui/x-ui.db
#   - vpn-productd running on 127.0.0.1:8080
#   - /etc/vpn-product/vpn-productd.env with VPN_PRODUCT_API_TOKEN
#
# Optional env:
#   SUB_TOKEN=...                # if set, binds subscription to this profile through API
#   PROFILE_ID=xui-test-vpn      # default profile id in vpn-product
#   PROFILE_NAME="VPN"           # default profile name
#   SERVER_IP=<public ip>        # force endpoint address, else autodetect
#   XUI_DB=/etc/x-ui/x-ui.db

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash deploy/scripts/sync_xui_to_product.sh"
  exit 1
fi

XUI_DB="${XUI_DB:-/etc/x-ui/x-ui.db}"
ENV_FILE="${ENV_FILE:-/etc/vpn-product/vpn-productd.env}"
API_URL="${API_URL:-http://127.0.0.1:8080}"
PROFILE_ID="${PROFILE_ID:-xui-test-vpn}"
PROFILE_NAME="${PROFILE_NAME:-VPN}"
STARTED_AT_UNIX="$(date +%s)"

if [[ ! -f "${XUI_DB}" ]]; then
  echo "Missing x-ui db: ${XUI_DB}"
  exit 1
fi
if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Missing env file: ${ENV_FILE}"
  exit 1
fi

# shellcheck disable=SC1090
source "${ENV_FILE}"
if [[ -z "${VPN_PRODUCT_API_TOKEN:-}" ]]; then
  echo "VPN_PRODUCT_API_TOKEN is empty in ${ENV_FILE}"
  exit 1
fi

sync_notify_failure() {
  curl -fsS \
    -H "Authorization: Bearer ${VPN_PRODUCT_API_TOKEN}" \
    -H "Content-Type: application/json" \
    -X POST "${API_URL}/v1/internal/sync/failure" \
    --data-binary '{"name":"xui_to_product"}' >/dev/null 2>&1 || true
}
trap 'sync_notify_failure' ERR

workdir="$(mktemp -d)"
trap 'rm -rf "${workdir}"' EXIT

python3 - "${XUI_DB}" <<'PY' > "${workdir}/xui.json"
import json, sqlite3, sys
db = sys.argv[1]
con = sqlite3.connect(db)
cur = con.cursor()
cur.execute("""SELECT id, remark, port, protocol, settings, stream_settings
               FROM inbounds
               WHERE COALESCE(enable,1)=1
               ORDER BY id DESC""")
rows = cur.fetchall()
picked = None
for r in rows:
    _id, remark, port, protocol, settings_raw, stream_raw = r
    if str(protocol).lower() != "vless":
        continue
    try:
        settings = json.loads(settings_raw or "{}")
    except Exception:
        settings = {}
    try:
        stream = json.loads(stream_raw or "{}")
    except Exception:
        stream = {}
    clients = settings.get("clients") or []
    if not clients:
        continue
    c0 = clients[0]
    rs = (stream.get("realitySettings") or {})
    rs_settings = (rs.get("settings") or {})
    short_ids = rs.get("shortIds") or []
    server_names = rs.get("serverNames") or []
    out = {
        "inbound_id": _id,
        "remark": remark or "TEST VPN",
        "port": int(port or 443),
        "uuid": c0.get("id", ""),
        "flow": c0.get("flow", "xtls-rprx-vision"),
        "public_key": rs_settings.get("publicKey", ""),
        "fingerprint": rs_settings.get("fingerprint", "chrome"),
        "server_name": rs_settings.get("serverName", (server_names[0] if server_names else "www.cloudflare.com")),
        "spider_x": rs_settings.get("spiderX", "/"),
        "short_id": (short_ids[0] if short_ids else ""),
    }
    if out["uuid"] and out["public_key"] and out["short_id"]:
        picked = out
        break

if not picked:
    print(json.dumps({"error": "no enabled VLESS+Reality inbound found in x-ui"}))
    sys.exit(2)

print(json.dumps(picked))
PY

if python3 - "${workdir}/xui.json" <<'PY'
import json, sys
obj = json.load(open(sys.argv[1], "r", encoding="utf-8"))
raise SystemExit(0 if "error" in obj else 1)
PY
then
  echo "No usable inbound in x-ui yet."
  echo "Create one in panel first: VLESS + Reality + 1 client."
  exit 1
fi

if [[ -n "${SERVER_IP:-}" ]]; then
  public_ip="${SERVER_IP}"
else
  public_ip="$(curl -fsS https://api.ipify.org || true)"
fi
if [[ -z "${public_ip}" ]]; then
  echo "Cannot detect public IP. Set SERVER_IP explicitly."
  exit 1
fi

python3 - "${workdir}/xui.json" "${PROFILE_ID}" "${PROFILE_NAME}" "${public_ip}" <<'PY' > "${workdir}/profile.json"
import json, sys
x = json.load(open(sys.argv[1], "r", encoding="utf-8"))
profile_id = sys.argv[2]
profile_name = sys.argv[3]
server_ip = sys.argv[4]
profile = {
    "id": profile_id,
    "name": profile_name,
    "description": "Synced from 3x-ui inbound id=%s" % x["inbound_id"],
    "enabled": True,
    "routeMode": "split",
    "endpoints": [
        {
            "name": "primary",
            "address": server_ip,
            "port": int(x["port"]),
            "protocol": "vless",
            "serverTag": "proxy",
            "uuid": x["uuid"],
            "flow": x.get("flow") or "xtls-rprx-vision",
            "serverName": x["server_name"],
            "fingerprint": x.get("fingerprint") or "chrome",
            "realityPublicKey": x["public_key"],
            "realityShortId": x["short_id"],
            "realityShortIds": [x["short_id"]],
            "realitySpiderX": x.get("spider_x") or "/",
            "realityServerNames": [x["server_name"]],
        }
    ],
    "preferredId": "primary",
    "fallback": {"endpointIds": ["primary"]},
    "dns": {"primary": ["https://1.1.1.1/dns-query"], "fallback": ["8.8.8.8"], "useDoH": True, "useDoQ": False, "queryIPv6": False},
    "reconnectPolicy": {"maxRetries": 3, "baseBackoff": 2000000000, "maxBackoff": 10000000000, "failureWindow": 60000000000, "degradedFailures": 2},
    "trafficLimitGb": 1024,
}
print(json.dumps(profile))
PY

curl -fsS \
  -H "Authorization: Bearer ${VPN_PRODUCT_API_TOKEN}" \
  -H "Content-Type: application/json" \
  -X POST "${API_URL}/v1/profiles/upsert" \
  --data-binary "@${workdir}/profile.json" >/dev/null

if [[ -n "${SUB_TOKEN:-}" ]]; then
  bind_payload="$(python3 - "${SUB_TOKEN}" "${PROFILE_ID}" <<'PY'
import json, sys
print(json.dumps({"token": sys.argv[1], "profileId": sys.argv[2]}))
PY
)"
  if ! curl -fsS \
    -H "Authorization: Bearer ${VPN_PRODUCT_API_TOKEN}" \
    -H "Content-Type: application/json" \
    -X POST "${API_URL}/v1/subscriptions/bind-profile" \
    --data-binary "${bind_payload}" >/dev/null; then
    echo "Warning: failed to bind SUB_TOKEN via API"
  fi
fi

heartbeat_payload="$(python3 - "${STARTED_AT_UNIX}" <<'PY'
import json, sys
print(json.dumps({"name": "xui_to_product", "startedAtUnix": int(sys.argv[1])}))
PY
)"
curl -fsS \
  -H "Authorization: Bearer ${VPN_PRODUCT_API_TOKEN}" \
  -H "Content-Type: application/json" \
  -X POST "${API_URL}/v1/internal/sync/heartbeat" \
  --data-binary "${heartbeat_payload}" >/dev/null || true

echo "Synced from 3x-ui to vpn-product."
echo "Profile: ${PROFILE_ID}"
if [[ -n "${SUB_TOKEN:-}" ]]; then
  echo "Subscription token bound: yes"
else
  echo "Subscription token bound: no (set SUB_TOKEN=...)"
fi
