#!/usr/bin/env bash
# Скачивает актуальные geoip.dat и geosite.dat (Loyalsoldier v2ray-rules-dat).
# Файлы размещаются в XRAY_DIR рядом с бинарником xray (типично /usr/local/x-ui/bin).

set -euo pipefail

usage() {
  echo "Download Loyalsoldier geoip.dat and geosite.dat into XRAY_DIR."
  echo "Usage: $0 [--help]"
  echo "Env: XRAY_DIR (default /usr/local/x-ui/bin), BACKUP_DIR (default /var/backups/vpn-product/geodata)"
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

XRAY_DIR="${XRAY_DIR:-/usr/local/x-ui/bin}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/vpn-product/geodata}"

GEOIP_URL="${GEOIP_URL:-https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat}"
GEOSITE_URL="${GEOSITE_URL:-https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat}"

mkdir -p "${BACKUP_DIR}"
mkdir -p "${XRAY_DIR}"

for f in geoip.dat geosite.dat; do
  if [[ -f "${XRAY_DIR}/${f}" ]]; then
    cp "${XRAY_DIR}/${f}" "${BACKUP_DIR}/${f}.$(date +%Y%m%d)"
  fi
done

curl -fsSL "${GEOIP_URL}" -o "${XRAY_DIR}/geoip.dat"
curl -fsSL "${GEOSITE_URL}" -o "${XRAY_DIR}/geosite.dat"

echo "[OK] geodata updated at $(date -u +%Y-%m-%dT%H:%M:%SZ) in ${XRAY_DIR}"
