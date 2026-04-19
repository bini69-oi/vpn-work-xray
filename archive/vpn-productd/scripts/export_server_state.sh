#!/usr/bin/env bash
set -euo pipefail

# Run on OLD server as root.
# Creates a tar.gz backup with DB/config/service state for migration.

OUT_DIR="${1:-/root/vpn-migration}"
STAMP="$(date -u +%Y%m%d-%H%M%S)"
WORK="${OUT_DIR}/state-${STAMP}"
ARCHIVE="${OUT_DIR}/vpn-migration-${STAMP}.tar.gz"

mkdir -p "${WORK}" "${OUT_DIR}"

copy_if_exists() {
  local src="$1"
  local dst="$2"
  if [[ -e "$src" ]]; then
    mkdir -p "$(dirname "${dst}")"
    cp -a "$src" "$dst"
  fi
}

# Core state
copy_if_exists /etc/vpn-product/vpn-productd.env "${WORK}/etc/vpn-product/vpn-productd.env"
copy_if_exists /var/lib/vpn-product/product.db "${WORK}/var/lib/vpn-product/product.db"
copy_if_exists /etc/x-ui/x-ui.db "${WORK}/etc/x-ui/x-ui.db"
copy_if_exists /etc/caddy/Caddyfile "${WORK}/etc/caddy/Caddyfile"

# Service units and sync scripts
copy_if_exists /etc/systemd/system/vpn-productd.service "${WORK}/etc/systemd/system/vpn-productd.service"
copy_if_exists /etc/systemd/system/vpn-xui-runtime-sync.service "${WORK}/etc/systemd/system/vpn-xui-runtime-sync.service"
copy_if_exists /etc/systemd/system/vpn-xui-runtime-sync.timer "${WORK}/etc/systemd/system/vpn-xui-runtime-sync.timer"
copy_if_exists /etc/systemd/system/vpn-xui-client-limits-sync.service "${WORK}/etc/systemd/system/vpn-xui-client-limits-sync.service"
copy_if_exists /etc/systemd/system/vpn-xui-client-limits-sync.timer "${WORK}/etc/systemd/system/vpn-xui-client-limits-sync.timer"
copy_if_exists /usr/local/bin/vpn-productd "${WORK}/usr/local/bin/vpn-productd"
copy_if_exists /usr/local/bin/vpn-xui-runtime-sync.sh "${WORK}/usr/local/bin/vpn-xui-runtime-sync.sh"
copy_if_exists /usr/local/bin/sync-xui-client-limits.py "${WORK}/usr/local/bin/sync-xui-client-limits.py"

# Metadata for quick restore checks
{
  echo "created_at_utc=$(date -u +%FT%TZ)"
  echo "hostname=$(hostname)"
  echo "kernel=$(uname -a)"
  echo "x_ui_status=$(systemctl is-active x-ui || true)"
  echo "vpn_productd_status=$(systemctl is-active vpn-productd || true)"
  echo "caddy_status=$(systemctl is-active caddy || true)"
} > "${WORK}/MIGRATION_INFO.env"

tar -C "${OUT_DIR}" -czf "${ARCHIVE}" "state-${STAMP}"
echo "ARCHIVE=${ARCHIVE}"
