#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash deploy/remnawave/scripts/install_node.sh"
  exit 1
fi

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo "[1/4] Installing Docker (official convenience script)."
curl -fsSL https://get.docker.com | sh

echo "[2/4] Creating RemnaNode directory."
mkdir -p /opt/remnanode

echo "[3/4] Installing node compose and env example."
install -m 0644 "${PROJECT_DIR}/deploy/remnawave/node/docker-compose.yml" /opt/remnanode/docker-compose.yml
install -m 0644 "${PROJECT_DIR}/deploy/remnawave/node/.env.example" /opt/remnanode/.env

echo "IMPORTANT: edit /opt/remnanode/.env (SECRET_KEY, NODE_PORT) before starting."
echo "Docs: https://docs.rw/docs/install/remnawave-node/"

echo "[4/4] Starting Remnawave Node container."
cd /opt/remnanode
docker compose up -d

echo "Node logs:"
echo "  docker compose -f /opt/remnanode/docker-compose.yml logs -f -t"
