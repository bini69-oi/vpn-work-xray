# VPN Product Staging Deploy Runbook

This runbook is intentionally single-server and systemd-based.

For incident response and fast troubleshooting, see `INCIDENT_RUNBOOK.md`.

## 1) Canonical pre-deploy checks

Run from repository root:

```bash
go test ./cmd/vpn-productd ./cmd/vpn-productctl
go test ./product/...
make test
```

Optional stricter checks:

```bash
make lint
make cover
go test ./...
```

## 2) Server prerequisites (Ubuntu LTS)

- Packages: `golang`, `ca-certificates`, `curl`, `git`, `iproute2`, `systemd`.
- User: `vpn-product` (system user, no shell).
- Directories:
  - `/etc/vpn-product`
  - `/var/lib/vpn-product`
  - `/var/log/vpn-product`
- Open local listen address from env (`VPN_PRODUCT_LISTEN`, default `127.0.0.1:8080`).
- If TUN is enabled in profiles, host must provide TUN support and routing permissions.

## 3) First staging deployment

1. Copy repository to server, for example `/opt/vpn-product/src`.
2. Run:

```bash
cd /opt/vpn-product/src
sudo bash deploy/scripts/deploy_staging.sh
```

3. Edit token and optional settings:

```bash
sudoedit /etc/vpn-product/vpn-productd.env
```

4. Restart service:

```bash
sudo systemctl restart vpn-productd
```

5. Smoke checks:

```bash
sudo bash /opt/vpn-product/src/deploy/scripts/smoke_staging.sh
```

## 4) Update existing staging install

```bash
cd /opt/vpn-product/src
git pull
sudo bash deploy/scripts/deploy_staging.sh
sudo bash deploy/scripts/smoke_staging.sh
```

## 5) Rollback

1. List backups:

```bash
ls -1 /opt/vpn-product/rollback
```

2. Restore previous binaries (example):

```bash
sudo cp /opt/vpn-product/rollback/vpn-productd.<stamp>.bak /usr/local/bin/vpn-productd
sudo cp /opt/vpn-product/rollback/vpn-productctl.<stamp>.bak /usr/local/bin/vpn-productctl
sudo systemctl restart vpn-productd
```

3. Re-run smoke script and verify:

```bash
sudo bash /opt/vpn-product/src/deploy/scripts/smoke_staging.sh
```

`/var/lib/vpn-product` (SQLite/data) is not destroyed by rollback.

## 6) Quick Happ + 3x-ui bootstrap (sslip)

Use this after `vpn-productd`, `xray`, and `caddy` are already installed and running:

```bash
cd /opt/vpn-product/src
sudo DOMAIN=198-13-186-187.sslip.io \
     XUI_DOMAIN=xui-198-13-186-187.sslip.io \
     SUB_TOKEN=CgNt6t8FJ6eabRnArv5hXDtmAxjV18zAzdn6o7FumnA \
     PANEL_USER=admin \
     PANEL_PASS='TestVpn_2026!' \
     bash deploy/scripts/bootstrap_happ_sslip.sh
```

Outputs:
- HTTPS subscription URL for Happ.
- HTTPS URL for 3x-ui panel.

## 7) One-command sync: 3x-ui -> vpn-product (for Telegram flow)

After you create/update inbound in 3x-ui, sync it into product profile and bind
your existing subscription token:

```bash
cd /opt/vpn-product/src
sudo SUB_TOKEN=CgNt6t8FJ6eabRnArv5hXDtmAxjV18zAzdn6o7FumnA \
     PROFILE_ID=xui-test-vpn \
     PROFILE_NAME="TEST VPN" \
     bash deploy/scripts/sync_xui_to_product.sh
```

This keeps one source of truth in panel and updates what Happ subscription returns.

### Fully automatic mode (no manual sync)

Install periodic sync timer (every 60s):

```bash
cd /opt/vpn-product/src
sudo bash deploy/scripts/install_sync_timer.sh
sudo systemctl status vpn-product-sync-xui.timer --no-pager
```

After this, you only change users/inbounds in 3x-ui panel, and subscription data is
synced automatically.

### Usage sync (x-ui traffic -> subscription counters)

```bash
cd /opt/vpn-product/src
sudo bash deploy/scripts/install_usage_sync_timer.sh
sudo systemctl status vpn-product-sync-usage.timer --no-pager
```

### Service-down Telegram alerts

```bash
cd /opt/vpn-product/src
sudo bash deploy/scripts/install_alert_timer.sh
sudoedit /etc/vpn-product/alerts.env
sudo systemctl restart vpn-product-service-alert.timer
```
