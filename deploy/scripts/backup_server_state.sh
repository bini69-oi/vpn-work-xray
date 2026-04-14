#!/usr/bin/env bash
set -euo pipefail

# Daily backup wrapper for vpn-product state.
# Usage:
#   BACKUP_DIR=/var/backups/vpn-product RETENTION_DAYS=14 \
#     /usr/local/bin/backup-server-state.sh

BACKUP_DIR="${BACKUP_DIR:-/var/backups/vpn-product}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"
EXPORT_SCRIPT="${EXPORT_SCRIPT:-}"

if [[ -z "${EXPORT_SCRIPT}" ]]; then
  for candidate in \
    /opt/vpn-product/src/deploy/scripts/export_server_state.sh \
    /opt/vpn-product/current/deploy/scripts/export_server_state.sh \
    /usr/local/bin/export_server_state.sh
  do
    if [[ -x "${candidate}" ]]; then
      EXPORT_SCRIPT="${candidate}"
      break
    fi
  done
fi

if [[ ! -x "${EXPORT_SCRIPT}" ]]; then
  echo "export script not found or not executable: ${EXPORT_SCRIPT}" >&2
  exit 1
fi

mkdir -p "${BACKUP_DIR}"
"${EXPORT_SCRIPT}" "${BACKUP_DIR}" >/tmp/vpn-product-backup.out

archive_path="$(sed -n 's/^ARCHIVE=//p' /tmp/vpn-product-backup.out | tail -n 1)"
if [[ -z "${archive_path}" || ! -f "${archive_path}" ]]; then
  echo "failed to produce backup archive" >&2
  cat /tmp/vpn-product-backup.out >&2 || true
  exit 1
fi

sha256sum "${archive_path}" > "${archive_path}.sha256"

BACKUP_ENCRYPT_KEY="${BACKUP_ENCRYPT_KEY:-}"
if [[ -n "${BACKUP_ENCRYPT_KEY}" ]]; then
    gpg --batch --yes --passphrase "${BACKUP_ENCRYPT_KEY}" \
        --symmetric --cipher-algo AES256 \
        -o "${archive_path}.gpg" "${archive_path}"
    rm -f "${archive_path}"
    archive_path="${archive_path}.gpg"
    sha256sum "${archive_path}" > "${archive_path}.sha256"
fi

# Keep recent backups only.
find "${BACKUP_DIR}" -type f -name 'vpn-migration-*.tar.gz' -mtime +"${RETENTION_DAYS}" -delete
find "${BACKUP_DIR}" -type f -name 'vpn-migration-*.tar.gz.sha256' -mtime +"${RETENTION_DAYS}" -delete

echo "backup_ok archive=${archive_path}"
