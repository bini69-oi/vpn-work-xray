#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash deploy/scripts/install_sync_timer.sh"
  exit 1
fi

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [[ "${PROJECT_DIR}/deploy/scripts/sync_xui_to_product.sh" != "/opt/vpn-product/src/deploy/scripts/sync_xui_to_product.sh" ]]; then
  install -m 0755 "${PROJECT_DIR}/deploy/scripts/sync_xui_to_product.sh" /opt/vpn-product/src/deploy/scripts/sync_xui_to_product.sh
else
  chmod 0755 /opt/vpn-product/src/deploy/scripts/sync_xui_to_product.sh
fi
install -m 0644 "${PROJECT_DIR}/deploy/systemd/vpn-product-sync-xui.service" /etc/systemd/system/vpn-product-sync-xui.service
install -m 0644 "${PROJECT_DIR}/deploy/systemd/vpn-product-sync-xui.timer" /etc/systemd/system/vpn-product-sync-xui.timer

if [[ ! -f /etc/vpn-product/sync-xui.env ]]; then
  install -m 0600 "${PROJECT_DIR}/deploy/env/sync-xui.env.example" /etc/vpn-product/sync-xui.env
  echo "Created /etc/vpn-product/sync-xui.env (edit SUB_TOKEN if needed)"
fi

systemctl daemon-reload
systemctl enable --now vpn-product-sync-xui.timer
systemctl start vpn-product-sync-xui.service || true

echo "Installed."
echo "Check status:"
echo "  systemctl status vpn-product-sync-xui.service --no-pager"
echo "  systemctl status vpn-product-sync-xui.timer --no-pager"

