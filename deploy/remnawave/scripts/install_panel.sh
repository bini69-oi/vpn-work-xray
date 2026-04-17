#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash deploy/remnawave/scripts/install_panel.sh"
  exit 1
fi

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo "[1/6] Installing Docker (official convenience script)."
curl -fsSL https://get.docker.com | sh

echo "[2/6] Creating Remnawave directories."
mkdir -p /opt/remnawave
mkdir -p /opt/remnawave/caddy

echo "[3/6] Installing panel compose and env examples."
install -m 0644 "${PROJECT_DIR}/deploy/remnawave/panel/docker-compose.yml" /opt/remnawave/docker-compose.yml
install -m 0644 "${PROJECT_DIR}/deploy/remnawave/panel/.env.example" /opt/remnawave/.env
install -m 0644 "${PROJECT_DIR}/deploy/remnawave/panel/caddy/Caddyfile" /opt/remnawave/caddy/Caddyfile

echo "IMPORTANT: edit /opt/remnawave/.env before starting."
echo "Docs: https://docs.rw/docs/install/remnawave-panel/"

echo "[4/6] Starting Remnawave Panel containers."
cd /opt/remnawave
docker compose up -d

echo "[5/6] Installing backup script + systemd timer (daily 00:00 UTC)."
install -m 0755 "${PROJECT_DIR}/deploy/remnawave/scripts/backup_panel.sh" /usr/local/bin/remnawave-backup.sh
install -m 0644 "${PROJECT_DIR}/deploy/remnawave/systemd/vpn-backup.service" /etc/systemd/system/vpn-backup.service
install -m 0644 "${PROJECT_DIR}/deploy/remnawave/systemd/vpn-backup.timer" /etc/systemd/system/vpn-backup.timer
mkdir -p /var/backups/remnawave

systemctl daemon-reload
systemctl enable --now vpn-backup.timer
systemctl start vpn-backup.service || true

echo "[6/6] Done."
echo "Panel logs:"
echo "  docker compose -f /opt/remnawave/docker-compose.yml logs -f -t"
echo "Backup status:"
echo "  systemctl status vpn-backup.timer --no-pager"
echo "  systemctl status vpn-backup.service --no-pager"
