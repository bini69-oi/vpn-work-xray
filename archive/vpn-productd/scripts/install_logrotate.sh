#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash deploy/scripts/install_logrotate.sh"
  exit 1
fi

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
install -m 0644 "${PROJECT_DIR}/deploy/logrotate/vpn-product" /etc/logrotate.d/vpn-product
logrotate -d /etc/logrotate.d/vpn-product >/dev/null
echo "Installed /etc/logrotate.d/vpn-product"
