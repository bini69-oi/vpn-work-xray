#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash scripts/install-ubuntu.sh"
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  apt-get update
  apt-get install -y golang ca-certificates curl git iproute2
fi

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_DIR"

LISTEN_ADDR="${VPN_PRODUCT_LISTEN:-127.0.0.1:8080}"
DATA_DIR="${VPN_PRODUCT_DATA_DIR:-/var/lib/vpn-product}"
INSTALL_3XUI="${INSTALL_3XUI:-0}"

go build -o /usr/local/bin/vpn-productd ./cmd/vpn-productd
go build -o /usr/local/bin/vpn-productctl ./cmd/vpn-productctl

mkdir -p /etc/vpn-product "$DATA_DIR"
if [[ ! -f /etc/vpn-product/token ]]; then
  head -c 32 /dev/urandom | base64 | tr -d '\n' > /etc/vpn-product/token
fi
TOKEN="$(cat /etc/vpn-product/token)"

cat >/etc/systemd/system/vpn-productd.service <<EOF
[Unit]
Description=VPN Product Daemon
After=network.target

[Service]
Type=simple
Environment=VPN_PRODUCT_API_TOKEN=${TOKEN}
ExecStart=/usr/local/bin/vpn-productd --listen ${LISTEN_ADDR} --data-dir ${DATA_DIR}
Restart=always
RestartSec=2
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF

cat >/etc/sysctl.d/99-vpn-product.conf <<EOF
fs.file-max = 1048576
net.core.somaxconn = 65535
net.core.netdev_max_backlog = 65535
net.ipv4.tcp_max_syn_backlog = 8192
net.ipv4.tcp_fin_timeout = 15
net.ipv4.ip_local_port_range = 10240 65535
net.ipv4.tcp_congestion_control = bbr
net.core.default_qdisc = fq
EOF

sysctl --system >/dev/null

ulimit -n 1048576 || true

systemctl daemon-reload
systemctl enable --now vpn-productd

if [[ "${INSTALL_3XUI}" == "1" ]] && ! command -v x-ui >/dev/null 2>&1; then
  bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
fi

echo "Installed."
echo "VPN token: $(cat /etc/vpn-product/token)"
echo "Daemon status: systemctl status vpn-productd --no-pager"
