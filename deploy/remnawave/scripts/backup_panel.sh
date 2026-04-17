#!/usr/bin/env bash
set -euo pipefail

# Remnawave Postgres backup:
# - pg_dump from remnawave-db container
# - store as tar.gz
# - write sha256 checksum
# - retention 14 days
#
# CHECK: verify DB container name is "remnawave-db" in your compose:
# https://raw.githubusercontent.com/remnawave/backend/refs/heads/main/docker-compose-prod.yml

BACKUP_DIR="${BACKUP_DIR:-/var/backups/remnawave}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"

mkdir -p "${BACKUP_DIR}"

ts="$(date -u +%Y%m%dT%H%M%SZ)"
workdir="$(mktemp -d)"
trap 'rm -rf "${workdir}"' EXIT

dump_path="${workdir}/remnawave-${ts}.sql"
archive_path="${BACKUP_DIR}/remnawave-${ts}.tar.gz"
sha_path="${archive_path}.sha256"

echo "Dumping database from container remnawave-db (UTC: ${ts})"
docker exec -i remnawave-db pg_dump -U postgres -d postgres > "${dump_path}"

tar -C "${workdir}" -czf "${archive_path}" "$(basename "${dump_path}")"
shasum -a 256 "${archive_path}" > "${sha_path}"

echo "Backup created:"
echo "  ${archive_path}"
echo "  ${sha_path}"

echo "Applying retention: ${RETENTION_DAYS} days"
find "${BACKUP_DIR}" -type f -name 'remnawave-*.tar.gz' -mtime "+${RETENTION_DAYS}" -delete
find "${BACKUP_DIR}" -type f -name 'remnawave-*.tar.gz.sha256' -mtime "+${RETENTION_DAYS}" -delete
