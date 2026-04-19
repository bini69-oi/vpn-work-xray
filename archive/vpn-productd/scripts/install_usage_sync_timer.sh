#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash deploy/scripts/install_usage_sync_timer.sh"
  exit 1
fi

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [[ "${PROJECT_DIR}/deploy/scripts/sync_xui_usage_to_product.sh" != "/opt/vpn-product/src/deploy/scripts/sync_xui_usage_to_product.sh" ]]; then
  install -m 0755 "${PROJECT_DIR}/deploy/scripts/sync_xui_usage_to_product.sh" /opt/vpn-product/src/deploy/scripts/sync_xui_usage_to_product.sh
else
  chmod 0755 /opt/vpn-product/src/deploy/scripts/sync_xui_usage_to_product.sh
fi
install -m 0644 "${PROJECT_DIR}/deploy/systemd/vpn-product-sync-usage.service" /etc/systemd/system/vpn-product-sync-usage.service
install -m 0644 "${PROJECT_DIR}/deploy/systemd/vpn-product-sync-usage.timer" /etc/systemd/system/vpn-product-sync-usage.timer

systemctl daemon-reload
systemctl enable --now vpn-product-sync-usage.timer
systemctl start vpn-product-sync-usage.service

echo "Installed usage sync timer."
echo "Check:"
echo "  systemctl status vpn-product-sync-usage.timer --no-pager"
