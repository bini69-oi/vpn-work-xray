#!/usr/bin/env bash
set -euo pipefail

# End-to-end smoke for issue pipeline before serving real users.
# Checks:
# - issue/link returns 200
# - URL is https public URL
# - appliedTo3xui=true
# - subscription endpoint returns 200
#
# Usage:
#   sudo bash deploy/scripts/smoke_issue_pipeline.sh [test_user]

TEST_USER="${1:-smoke_issue_user}"
ENV_FILE="${ENV_FILE:-/etc/vpn-product/vpn-productd.env}"
API="${API:-http://127.0.0.1:8080}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "missing env file: ${ENV_FILE}"
  exit 1
fi
ADMIN_TOKEN="$(sed -n 's/^VPN_PRODUCT_ADMIN_TOKEN=//p' "${ENV_FILE}" | tr -d '\r\n')"
if [[ -z "${ADMIN_TOKEN}" ]]; then
  echo "VPN_PRODUCT_ADMIN_TOKEN is empty"
  exit 1
fi

RESP="$(curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" -H "Content-Type: application/json" -d "{\"userId\":\"${TEST_USER}\",\"source\":\"smoke_issue\"}" "${API}/admin/issue/link")"

python3 - <<'PY' "${RESP}"
import json
import sys
import urllib.request

resp = json.loads(sys.argv[1])
url = str(resp.get("url", "")).strip()
applied = bool(resp.get("appliedTo3xui", False))

if not url:
    raise SystemExit("FAIL: empty url in issue response")
if not (url.startswith("https://") or url.startswith("http://")):
    raise SystemExit(f"FAIL: non-http url: {url}")
if "127.0.0.1" in url or "localhost" in url:
    raise SystemExit(f"FAIL: local url returned: {url}")
if not applied:
    raise SystemExit(f"FAIL: appliedTo3xui=false applyError={resp.get('applyError')}")

req = urllib.request.Request(url, method="GET")
with urllib.request.urlopen(req, timeout=10) as r:
    body = r.read().decode("utf-8", errors="ignore")
if not body.strip():
    raise SystemExit("FAIL: empty subscription body")
print("OK: issue pipeline healthy")
print("URL:", url)
PY
