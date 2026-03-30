#!/usr/bin/env bash
set -euo pipefail

ALERT_ENV="${ALERT_ENV:-/etc/vpn-product/alerts.env}"

if [[ ! -f "${ALERT_ENV}" ]]; then
  exit 0
fi

# shellcheck disable=SC1090
source "${ALERT_ENV}"

if [[ -z "${TG_BOT_TOKEN:-}" || -z "${TG_CHAT_ID:-}" ]]; then
  exit 0
fi

SERVICES=("x-ui" "vpn-productd" "caddy")
failed=()
for svc in "${SERVICES[@]}"; do
  if ! systemctl is-active --quiet "${svc}"; then
    failed+=("${svc}")
  fi
done

if [[ "${#failed[@]}" -eq 0 ]]; then
  exit 0
fi

host="$(hostname -f 2>/dev/null || hostname)"
msg="🚨 VPN ALERT on ${host}%0AInactive services: ${failed[*]}"
curl -fsS -X POST "https://api.telegram.org/bot${TG_BOT_TOKEN}/sendMessage" \
  -d "chat_id=${TG_CHAT_ID}" \
  -d "text=${msg}" >/dev/null || true
