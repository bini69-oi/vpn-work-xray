#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash deploy/scripts/install_geodata_timer.sh"
  exit 1
fi

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  echo "Installs vpn-geodata-update.service and .timer for daily geo file refresh."
  exit 0
fi

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

mkdir -p /opt/vpn-product/src/deploy/scripts
install -m 0755 "${PROJECT_DIR}/deploy/scripts/update_geodata.sh" /opt/vpn-product/src/deploy/scripts/update_geodata.sh
install -m 0644 "${PROJECT_DIR}/deploy/systemd/vpn-geodata-update.service" /etc/systemd/system/vpn-geodata-update.service
install -m 0644 "${PROJECT_DIR}/deploy/systemd/vpn-geodata-update.timer" /etc/systemd/system/vpn-geodata-update.timer

systemctl daemon-reload
systemctl enable --now vpn-geodata-update.timer
systemctl start vpn-geodata-update.service || true

echo "Installed."
echo "  systemctl status vpn-geodata-update.timer --no-pager"
