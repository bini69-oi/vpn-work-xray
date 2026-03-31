#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ASSET_DIR="${XRAY_LOCATION_ASSET:-${ROOT_DIR}/var/vpn-product-predeploy3/assets}"
RESOURCES_DIR="${ROOT_DIR}/resources"

mkdir -p "${RESOURCES_DIR}"

for file in geoip.dat geosite.dat; do
  if [[ -f "${RESOURCES_DIR}/${file}" ]]; then
    continue
  fi
  if [[ ! -f "${ASSET_DIR}/${file}" ]]; then
    echo "missing required test asset: ${ASSET_DIR}/${file}"
    exit 1
  fi
  cp "${ASSET_DIR}/${file}" "${RESOURCES_DIR}/${file}"
done

echo "prepared test assets in ${RESOURCES_DIR}"
