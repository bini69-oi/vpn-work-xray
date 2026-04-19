#!/usr/bin/env bash
set -euo pipefail

# Daily lifecycle sync:
# - blocks expired subscriptions
# - renews expired subscriptions for users listed in AUTO_RENEW_USERS
# Uses vpn-product lifecycle API so changes are synced to both vpn-product and 3x-ui.

ENV_FILE="${ENV_FILE:-/etc/vpn-product/vpn-productd.env}"
PDB="${PDB:-/var/lib/vpn-product/product.db}"
API_URL="${API_URL:-http://127.0.0.1:8080}"
RENEW_DAYS="${RENEW_DAYS:-30}"
AUTO_RENEW_USERS_RAW="${AUTO_RENEW_USERS:-}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "missing env file: ${ENV_FILE}"
  exit 1
fi

ADMIN_TOKEN="$(sed -n 's/^VPN_PRODUCT_ADMIN_TOKEN=//p' "${ENV_FILE}" | tr -d '\r\n')"
if [[ -z "${ADMIN_TOKEN}" ]]; then
  echo "VPN_PRODUCT_ADMIN_TOKEN is empty"
  exit 1
fi

if [[ ! -f "${PDB}" ]]; then
  echo "missing product db: ${PDB}"
  exit 1
fi

python3 - <<'PY' "${PDB}" "${API_URL}" "${ADMIN_TOKEN}" "${RENEW_DAYS}" "${AUTO_RENEW_USERS_RAW}"
import json
import sqlite3
import sys
import urllib.request
from datetime import datetime, timezone

pdb, api_url, token, renew_days, renew_users_raw = sys.argv[1:]
renew_days_i = int(renew_days)
renew_users = {u.strip() for u in renew_users_raw.split(",") if u.strip()}

def parse_iso(value: str):
    if not value:
        return None
    val = value.strip().replace("Z", "+00:00")
    try:
        dt = datetime.fromisoformat(val)
    except ValueError:
        return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(timezone.utc)

def post_json(path: str, payload: dict):
    body = json.dumps(payload).encode()
    req = urllib.request.Request(
        f"{api_url}{path}",
        data=body,
        headers={
            "Authorization": f"Bearer {token}",
            "Content-Type": "application/json",
        },
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=20) as resp:
        return resp.getcode(), resp.read().decode("utf-8", errors="ignore")

conn = sqlite3.connect(pdb)
cur = conn.cursor()
rows = cur.execute(
    "SELECT user_id, expires_at FROM subscriptions WHERE revoked=0 AND status='active'"
).fetchall()
conn.close()

now = datetime.now(timezone.utc)
actions = []
for user_id, expires_at in rows:
    user = (user_id or "").strip()
    if not user:
        continue
    exp = parse_iso(expires_at or "")
    if exp is None or exp > now:
        continue
    if user in renew_users:
        actions.append(("renew", user))
    else:
        actions.append(("block", user))

if not actions:
    print("no expired subscriptions to process")
    raise SystemExit(0)

for action, user in actions:
    payload = {"userId": user, "action": action}
    if action == "renew":
        payload["days"] = renew_days_i
    code, body = post_json("/admin/subscriptions/lifecycle", payload)
    print(f"{action} user={user} http={code} body={body[:200]}")
PY
