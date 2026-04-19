#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RESOURCES_DIR="${ROOT_DIR}/resources"
GEOIP_URL="${XRAY_TEST_GEOIP_URL:-https://github.com/v2fly/geoip/releases/latest/download/geoip.dat}"
GEOSITE_URL="${XRAY_TEST_GEOSITE_URL:-https://github.com/v2fly/domain-list-community/releases/latest/download/dlc.dat}"
ASSET_DIR="${XRAY_LOCATION_ASSET:-}"

mkdir -p "${RESOURCES_DIR}"

resolve_asset_dir() {
  local preferred="$1"
  local candidates=()
  if [[ -n "${preferred}" ]]; then
    candidates+=("${preferred}")
  fi
  candidates+=(
    "${ROOT_DIR}/var/vpn-product-predeploy3/assets"
    "${ROOT_DIR}/var/vpn-product-predeploy2/assets"
  )
  for candidate in "${candidates[@]}"; do
    if [[ -f "${candidate}/geoip.dat" && -f "${candidate}/geosite.dat" ]]; then
      echo "${candidate}"
      return 0
    fi
  done
  return 1
}

if resolved="$(resolve_asset_dir "${ASSET_DIR}")"; then
  ASSET_DIR="${resolved}"
else
  ASSET_DIR=""
fi

for file in geoip.dat geosite.dat; do
  if [[ -f "${RESOURCES_DIR}/${file}" ]]; then
    continue
  fi
  if [[ -n "${ASSET_DIR}" && -f "${ASSET_DIR}/${file}" ]]; then
    cp "${ASSET_DIR}/${file}" "${RESOURCES_DIR}/${file}"
    continue
  fi
  if [[ ! -f "${RESOURCES_DIR}/${file}" ]]; then
    url="${GEOIP_URL}"
    if [[ "${file}" == "geosite.dat" ]]; then
      url="${GEOSITE_URL}"
    fi
    echo "missing local asset for ${file}; downloading from ${url}"
    curl --retry 5 --retry-delay 2 --retry-connrefused --connect-timeout 15 --max-time 180 -fsSL "${url}" -o "${RESOURCES_DIR}/${file}"
  fi
done

echo "prepared test assets in ${RESOURCES_DIR}"
