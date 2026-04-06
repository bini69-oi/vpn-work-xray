#!/usr/bin/env bash
# Installs Prometheus + Grafana via docker compose (deploy/docker-compose.monitoring.yml).
# Grafana UI is bound to 0.0.0.0:3000 — restrict with firewall on production hosts.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required" >&2
  exit 1
fi

compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  else
    docker-compose "$@"
  fi
}

compose -f docker-compose.monitoring.yml up -d

echo "[OK] Prometheus: http://127.0.0.1:9090"
echo "[OK] Grafana:    http://127.0.0.1:3000  (user admin; set GRAFANA_PASSWORD or use default admin)"

if command -v ufw >/dev/null 2>&1; then
  echo "Tip: sudo ufw allow from <admin-ip> to any port 3000 proto tcp"
fi
