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

XUI_PORT="${XUI_PORT:-8443}"

if [[ ! -f "${XUI_DB}" ]]; then
  echo "Missing x-ui db: ${XUI_DB}"
  exit 1
fi
if [[ ! -f "${PRODUCT_DB}" ]]; then
  echo "Missing product db: ${PRODUCT_DB}"
  exit 1
fi

python3 - "${XUI_DB}" "${PRODUCT_DB}" "${XUI_PORT}" <<'PY'
import sqlite3, sys, json, re, time
xui_db, product_db, port = sys.argv[1], sys.argv[2], int(sys.argv[3])
now = time.strftime("%Y-%m-%dT%H:%M:%S.000000000Z", time.gmtime())

def sanitize_id(raw: str) -> str:
    clean = re.sub(r"[^a-zA-Z0-9_-]+", "-", raw.strip())
    return clean.strip("-").lower()[:64] or "unknown"

xcon = sqlite3.connect(xui_db)
xcur = xcon.cursor()
row = xcur.execute(
    "SELECT id FROM inbounds WHERE protocol='vless' AND port=? ORDER BY id DESC LIMIT 1",
    (port,),
).fetchone()
if not row:
    print("no inbound found")
    sys.exit(0)
inbound_id = row[0]
traffic_rows = xcur.execute(
    "SELECT email, COALESCE(up,0), COALESCE(down,0) FROM client_traffics WHERE inbound_id=?",
    (inbound_id,),
).fetchall()
xcon.close()

pcon = sqlite3.connect(product_db)
pcur = pcon.cursor()
updated = 0
for email, up, down in traffic_rows:
    email = (email or "").strip()
    if not email:
        continue
    profile_id = "user-" + sanitize_id(email)
    total = int(up) + int(down)
    pcur.execute(
        """
        INSERT INTO profile_quota(profile_id, limit_mb, used_upload_bytes, used_download_bytes, expires_at, traffic_limit_gb, traffic_used_bytes, blocked, updated_at)
        VALUES(?, 0, ?, ?, NULL, 1024, ?, 0, ?)
        ON CONFLICT(profile_id) DO UPDATE SET
          used_upload_bytes=excluded.used_upload_bytes,
          used_download_bytes=excluded.used_download_bytes,
          traffic_used_bytes=excluded.traffic_used_bytes,
          updated_at=excluded.updated_at
        """,
        (profile_id, int(up), int(down), total, now),
    )
    updated += 1
pcon.commit()
pcon.close()
print(f"updated_profiles={updated}")
PY
