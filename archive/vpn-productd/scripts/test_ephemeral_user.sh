#!/usr/bin/env bash
set -euo pipefail

# One-shot smoke flow with disposable user:
# 1) issue link
# 2) validate subscription endpoint
# 3) cleanup test user from product/x-ui DB

API="${API:-http://127.0.0.1:8080}"
ENV_FILE="${ENV_FILE:-/etc/vpn-product/vpn-productd.env}"
TEST_USER="${TEST_USER:-test_once_user}"
PDB="${PDB:-/var/lib/vpn-product/product.db}"
XDB="${XDB:-/etc/x-ui/x-ui.db}"

ADMIN_TOKEN="$(sed -n 's/^VPN_PRODUCT_ADMIN_TOKEN=//p' "$ENV_FILE" | tr -d '\r\n')"
if [[ -z "${ADMIN_TOKEN}" ]]; then
  echo "VPN_PRODUCT_ADMIN_TOKEN is empty"
  exit 1
fi

echo "[1/3] issue link for ${TEST_USER}"
RESP="$(curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" -H "Content-Type: application/json" \
  -d "{\"userId\":\"${TEST_USER}\",\"source\":\"smoke\"}" \
  "${API}/admin/issue/link")"
URL="$(printf '%s' "$RESP" | sed -n 's/.*"url":"\([^"]*\)".*/\1/p')"
TOKEN="${URL##*/}"

echo "[2/3] validate subscription endpoint"
CODE="$(curl -s -o /tmp/sub_ephemeral.txt -w '%{http_code}' "${API}/public/subscriptions/${TOKEN}")"
if [[ "$CODE" != "200" ]]; then
  echo "subscription check failed: HTTP ${CODE}"
  exit 1
fi
sed -n '1p' /tmp/sub_ephemeral.txt

echo "[3/3] cleanup disposable user ${TEST_USER}"
python3 - <<PY
import sqlite3
u = "${TEST_USER}"

pc = sqlite3.connect("${PDB}")
cur = pc.cursor()
cur.execute("UPDATE subscriptions SET revoked=1, revoked_at=datetime('now'), status='revoked', updated_at=datetime('now') WHERE user_id=?", (u,))
cur.execute("DELETE FROM panel_users WHERE external_id=?", (u,))
cur.execute("DELETE FROM profile_quota WHERE profile_id=?", ("user-" + u.replace("_", "-"),))
cur.execute("DELETE FROM profiles WHERE id=?", ("user-" + u.replace("_", "-"),))
pc.commit()
pc.close()

xc = sqlite3.connect("${XDB}")
cur = xc.cursor()
cur.execute("DELETE FROM client_traffics WHERE email=?", (u,))
row = cur.execute("SELECT id, settings FROM inbounds WHERE protocol='vless' AND port=8443 ORDER BY id DESC LIMIT 1").fetchone()
if row:
    import json
    inbound_id, settings_raw = row
    settings = json.loads(settings_raw or "{}")
    settings["clients"] = [c for c in settings.get("clients", []) if str(c.get("email","")).strip() != u]
    cur.execute("UPDATE inbounds SET settings=? WHERE id=?", (json.dumps(settings, separators=(",", ":")), inbound_id))
xc.commit()
xc.close()
print("cleanup done")
PY

systemctl restart x-ui >/dev/null 2>&1 || true
echo "ok"
