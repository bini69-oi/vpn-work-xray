#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash deploy/scripts/deploy_staging.sh"
  exit 1
fi

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RELEASE_ROOT="/opt/vpn-product/releases"
CURRENT_LINK="/opt/vpn-product/current"
ROLLBACK_ROOT="/opt/vpn-product/rollback"
STAMP="$(date +%Y%m%d%H%M%S)"
RELEASE_DIR="${RELEASE_ROOT}/${STAMP}"

mkdir -p "${RELEASE_DIR}/bin" "${ROLLBACK_ROOT}" /etc/vpn-product /var/lib/vpn-product /var/log/vpn-product

if ! id -u vpn-product >/dev/null 2>&1; then
  useradd --system --home /var/lib/vpn-product --shell /usr/sbin/nologin vpn-product
fi

chown -R vpn-product:vpn-product /var/lib/vpn-product /var/log/vpn-product

echo "Building binaries from ${PROJECT_DIR}"
cd "${PROJECT_DIR}"
go test ./cmd/vpn-productd ./cmd/vpn-productctl >/dev/null
go test ./product/... >/dev/null
go build -o "${RELEASE_DIR}/bin/vpn-productd" ./cmd/vpn-productd
go build -o "${RELEASE_DIR}/bin/vpn-productctl" ./cmd/vpn-productctl

if [[ -f /usr/local/bin/vpn-productd ]]; then
  cp /usr/local/bin/vpn-productd "${ROLLBACK_ROOT}/vpn-productd.${STAMP}.bak"
fi
if [[ -f /usr/local/bin/vpn-productctl ]]; then
  cp /usr/local/bin/vpn-productctl "${ROLLBACK_ROOT}/vpn-productctl.${STAMP}.bak"
fi

install -m 0755 "${RELEASE_DIR}/bin/vpn-productd" /usr/local/bin/vpn-productd
install -m 0755 "${RELEASE_DIR}/bin/vpn-productctl" /usr/local/bin/vpn-productctl

ln -sfn "${RELEASE_DIR}" "${CURRENT_LINK}"

if [[ ! -f /etc/vpn-product/vpn-productd.env ]]; then
  cp "${PROJECT_DIR}/deploy/env/vpn-productd.env.example" /etc/vpn-product/vpn-productd.env
  chmod 0600 /etc/vpn-product/vpn-productd.env
  echo "Created /etc/vpn-product/vpn-productd.env; edit token before start."
fi

install -m 0644 "${PROJECT_DIR}/deploy/systemd/vpn-productd.service" /etc/systemd/system/vpn-productd.service

systemctl daemon-reload
systemctl enable vpn-productd
systemctl restart vpn-productd

echo "Deployment done. Run smoke checks:"
echo "  sudo bash ${PROJECT_DIR}/deploy/scripts/smoke_staging.sh"
