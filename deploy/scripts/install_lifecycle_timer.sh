#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash deploy/scripts/install_lifecycle_timer.sh"
  exit 1
fi

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

install -m 0755 "${PROJECT_DIR}/deploy/scripts/lifecycle_daily_sync.sh" /usr/local/bin/lifecycle-daily-sync.sh

install -m 0644 "${PROJECT_DIR}/deploy/systemd/vpn-product-lifecycle-daily.service" /etc/systemd/system/vpn-product-lifecycle-daily.service
install -m 0644 "${PROJECT_DIR}/deploy/systemd/vpn-product-lifecycle-daily.timer" /etc/systemd/system/vpn-product-lifecycle-daily.timer

if [[ ! -f /etc/vpn-product/lifecycle-sync.env ]]; then
  install -m 0600 "${PROJECT_DIR}/deploy/env/lifecycle-sync.env.example" /etc/vpn-product/lifecycle-sync.env
  echo "Created /etc/vpn-product/lifecycle-sync.env (edit AUTO_RENEW_USERS if needed)"
fi

systemctl daemon-reload
systemctl enable --now vpn-product-lifecycle-daily.timer
systemctl start vpn-product-lifecycle-daily.service || true

echo "Installed lifecycle timer."
echo "Check:"
echo "  systemctl status vpn-product-lifecycle-daily.timer --no-pager"
