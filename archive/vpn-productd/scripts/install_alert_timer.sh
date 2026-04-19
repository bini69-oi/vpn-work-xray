#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash deploy/scripts/install_alert_timer.sh"
  exit 1
fi

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [[ "${PROJECT_DIR}/deploy/scripts/check_services_alert.sh" != "/opt/vpn-product/src/deploy/scripts/check_services_alert.sh" ]]; then
  install -m 0755 "${PROJECT_DIR}/deploy/scripts/check_services_alert.sh" /opt/vpn-product/src/deploy/scripts/check_services_alert.sh
else
  chmod 0755 /opt/vpn-product/src/deploy/scripts/check_services_alert.sh
fi
install -m 0644 "${PROJECT_DIR}/deploy/systemd/vpn-product-service-alert.service" /etc/systemd/system/vpn-product-service-alert.service
install -m 0644 "${PROJECT_DIR}/deploy/systemd/vpn-product-service-alert.timer" /etc/systemd/system/vpn-product-service-alert.timer

if [[ ! -f /etc/vpn-product/alerts.env ]]; then
  install -m 0600 "${PROJECT_DIR}/deploy/env/alerts.env.example" /etc/vpn-product/alerts.env
  echo "Created /etc/vpn-product/alerts.env (set TG_BOT_TOKEN and TG_CHAT_ID)"
fi

systemctl daemon-reload
systemctl enable --now vpn-product-service-alert.timer
echo "Alert timer installed."
