#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo bash deploy/scripts/bootstrap_happ_sslip.sh"
  exit 1
fi

DOMAIN="${DOMAIN:-198-13-186-187.sslip.io}"
XUI_DOMAIN="${XUI_DOMAIN:-xui-198-13-186-187.sslip.io}"
SUB_TOKEN="${SUB_TOKEN:-}"
PANEL_USER="${PANEL_USER:-admin}"
PANEL_PASS="${PANEL_PASS:-}"
PANEL_BASE_PATH="${PANEL_BASE_PATH:-/vpnpanel/}"

if ! command -v caddy >/dev/null 2>&1; then
  echo "caddy is required"
  exit 1
fi

if ! command -v x-ui >/dev/null 2>&1; then
  echo "Installing 3x-ui..."
  bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
fi

if [[ -z "${SUB_TOKEN}" ]]; then
  echo "SUB_TOKEN is required (do not use hardcoded token defaults)"
  exit 1
fi
if [[ -z "${PANEL_PASS}" ]]; then
  echo "PANEL_PASS is required (do not use hardcoded password defaults)"
  exit 1
fi

install -m 0644 /usr/local/x-ui/x-ui.service.debian /etc/systemd/system/x-ui.service
systemctl daemon-reload
systemctl enable --now x-ui

/usr/local/x-ui/x-ui setting -username "${PANEL_USER}" -password "${PANEL_PASS}"
/usr/local/x-ui/x-ui setting -webBasePath "${PANEL_BASE_PATH}"
/usr/local/x-ui/x-ui setting -listenIP 127.0.0.1
# Avoid default predictable subscription URI warning in panel.
python3 - <<'PY'
import sqlite3, secrets
db = "/etc/x-ui/x-ui.db"
con = sqlite3.connect(db)
cur = con.cursor()
cur.execute("SELECT COUNT(*) FROM settings WHERE key=?", ("subURI",))
exists = cur.fetchone()[0] > 0
value = "/" + secrets.token_urlsafe(24) + "/"
if exists:
    cur.execute("UPDATE settings SET value=? WHERE key=?", (value, "subURI"))
else:
    cur.execute("INSERT INTO settings(key, value) VALUES(?, ?)", ("subURI", value))
con.commit()
print("subURI=", value)
PY
x-ui restart

cat > /etc/caddy/Caddyfile <<EOF
${DOMAIN} {
    encode zstd gzip
    reverse_proxy 127.0.0.1:8080
}

${XUI_DOMAIN} {
    encode zstd gzip
    reverse_proxy 127.0.0.1:13554
}
EOF

caddy validate --config /etc/caddy/Caddyfile
systemctl reload caddy

echo "Done."
echo "Happ subscription URL:"
echo "https://${DOMAIN}/public/subscriptions/${SUB_TOKEN}"
echo
echo "3x-ui panel:"
echo "https://${XUI_DOMAIN}${PANEL_BASE_PATH}"
echo "user=${PANEL_USER}"
echo "pass=${PANEL_PASS}"
