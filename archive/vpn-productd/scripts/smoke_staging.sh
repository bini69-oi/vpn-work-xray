#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${ENV_FILE:-/etc/vpn-product/vpn-productd.env}"
API_URL="${API_URL:-http://127.0.0.1:8080}"

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

auth_header="Authorization: Bearer ${VPN_PRODUCT_API_TOKEN}"
admin_token="${VPN_PRODUCT_ADMIN_TOKEN:-${VPN_PRODUCT_API_TOKEN}}"
admin_auth_header="Authorization: Bearer ${admin_token}"

expect_code() {
  local code="$1"
  local expected="$2"
  local label="$3"
  if [[ "${code}" != "${expected}" ]]; then
    echo "[FAIL] ${label}: expected ${expected}, got ${code}"
    exit 1
  fi
  echo "[OK] ${label}: ${code}"
}

echo "== Systemd =="
systemctl is-active --quiet vpn-productd
echo "[OK] vpn-productd is active"

echo "== API auth =="
code="$(curl -s -o /tmp/vpn_smoke_noauth.json -w "%{http_code}" "${API_URL}/v1/status")"
expect_code "${code}" "401" "status without auth"

code="$(curl -s -o /tmp/vpn_smoke_status.json -w "%{http_code}" -H "${auth_header}" "${API_URL}/v1/status")"
expect_code "${code}" "200" "status with auth"

echo "== Health/Readiness/Metrics =="
health_code="$(curl -s -o /tmp/vpn_smoke_health.json -w "%{http_code}" -H "${auth_header}" "${API_URL}/v1/health")"
if [[ "${health_code}" != "200" && "${health_code}" != "503" ]]; then
  echo "[FAIL] health expected 200 or 503, got ${health_code}"
  exit 1
fi
echo "[OK] health endpoint returned ${health_code}"

readiness_code="$(curl -s -o /tmp/vpn_smoke_readiness.json -w "%{http_code}" -H "${admin_auth_header}" "${API_URL}/admin/readiness")"
expect_code "${readiness_code}" "200" "admin readiness"

code="$(curl -s -o /tmp/vpn_smoke_metrics.txt -w "%{http_code}" -H "${auth_header}" "${API_URL}/v1/metrics")"
expect_code "${code}" "200" "metrics with auth"

echo "== SQLite runtime artifacts =="
if [[ -n "${VPN_PRODUCT_DATA_DIR:-}" ]]; then
  if [[ ! -f "${VPN_PRODUCT_DATA_DIR}/product.db" ]]; then
    echo "[FAIL] missing SQLite db: ${VPN_PRODUCT_DATA_DIR}/product.db"
    exit 1
  fi
  echo "[OK] SQLite DB exists"
fi

echo "== CLI basic =="
vpn-productctl --api "${API_URL}" --token "${VPN_PRODUCT_API_TOKEN}" status >/tmp/vpn_smoke_ctl_status.json
vpn-productctl --api "${API_URL}" --token "${VPN_PRODUCT_API_TOKEN}" profiles >/tmp/vpn_smoke_ctl_profiles.json
echo "[OK] vpn-productctl status/profiles"

echo "== Failure path checks =="
code="$(curl -s -o /tmp/vpn_smoke_badtoken.json -w "%{http_code}" -H "Authorization: Bearer wrong-token" "${API_URL}/v1/status")"
expect_code "${code}" "401" "status with wrong token"

code="$(curl -s -o /tmp/vpn_smoke_badprofile.json -w "%{http_code}" -H "${auth_header}" -H "Content-Type: application/json" -d '{"profileId":"missing-profile"}' "${API_URL}/v1/connect")"
if [[ "${code}" != "400" && "${code}" != "404" ]]; then
  echo "[FAIL] connect invalid profile expected 400/404, got ${code}"
  exit 1
fi
echo "[OK] connect invalid profile returned ${code}"

echo "Smoke checks passed."
