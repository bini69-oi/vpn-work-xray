#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash deploy/scripts/sync_xui_usage_to_product.sh"
  exit 1
fi

SYNC_ENV="${SYNC_ENV:-/etc/vpn-product/sync-xui.env}"
XUI_DB="${XUI_DB:-/etc/x-ui/x-ui.db}"
PRODUCT_DB="${PRODUCT_DB:-/var/lib/vpn-product/product.db}"

if [[ -f "${SYNC_ENV}" ]]; then
  # shellcheck disable=SC1090
  source "${SYNC_ENV}"
fi

PROFILE_ID="${PROFILE_ID:-xui-test-vpn}"
XUI_PORT="${XUI_PORT:-8443}"

if [[ ! -f "${XUI_DB}" ]]; then
  echo "Missing x-ui db: ${XUI_DB}"
  exit 1
fi
if [[ ! -f "${PRODUCT_DB}" ]]; then
  echo "Missing product db: ${PRODUCT_DB}"
  exit 1
fi

read -r up down <<<"$(python3 - "${XUI_DB}" "${XUI_PORT}" <<'PY'
import sqlite3, sys
db = sys.argv[1]
port = int(sys.argv[2])
con = sqlite3.connect(db)
cur = con.cursor()
cur.execute("SELECT COALESCE(up,0), COALESCE(down,0) FROM inbounds WHERE port=? ORDER BY id DESC LIMIT 1", (port,))
row = cur.fetchone()
if not row:
    print("0 0")
else:
    print(f"{int(row[0])} {int(row[1])}")
PY
)"

total=$((up + down))
now="$(date -u +"%Y-%m-%dT%H:%M:%S.000000000Z")"

python3 - "${PRODUCT_DB}" "${PROFILE_ID}" "${up}" "${down}" "${total}" "${now}" <<'PY'
import sqlite3, sys
db, profile_id, up, down, total, now = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4]), int(sys.argv[5]), sys.argv[6]
con = sqlite3.connect(db)
cur = con.cursor()
cur.execute(
    """
    INSERT INTO profile_quota(profile_id, limit_mb, used_upload_bytes, used_download_bytes, expires_at, traffic_limit_gb, traffic_used_bytes, blocked, updated_at)
    VALUES(?, 0, ?, ?, NULL, 1024, ?, 0, ?)
    ON CONFLICT(profile_id) DO UPDATE SET
      used_upload_bytes=excluded.used_upload_bytes,
      used_download_bytes=excluded.used_download_bytes,
      traffic_used_bytes=excluded.traffic_used_bytes,
      updated_at=excluded.updated_at
    """,
    (profile_id, up, down, total, now),
)
con.commit()
print("ok")
PY

echo "Synced usage: profile=${PROFILE_ID} up=${up} down=${down} total=${total}"
