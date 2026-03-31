#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash deploy/scripts/install_backup_timer.sh"
  exit 1
fi

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

install -m 0755 "${PROJECT_DIR}/deploy/scripts/backup_server_state.sh" /usr/local/bin/backup-server-state.sh
install -m 0644 "${PROJECT_DIR}/deploy/systemd/vpn-product-backup.service" /etc/systemd/system/vpn-product-backup.service
install -m 0644 "${PROJECT_DIR}/deploy/systemd/vpn-product-backup.timer" /etc/systemd/system/vpn-product-backup.timer
mkdir -p /var/backups/vpn-product

systemctl daemon-reload
systemctl enable --now vpn-product-backup.timer

# Trigger one immediate backup so operators can verify output.
systemctl start vpn-product-backup.service || true

echo "Installed daily backup timer."
echo "Check status:"
echo "  systemctl status vpn-product-backup.timer --no-pager"
echo "  systemctl status vpn-product-backup.service --no-pager"
echo "List backups:"
echo "  ls -lah /var/backups/vpn-product"
