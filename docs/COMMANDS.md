# Commands / how to run everything

This file is a practical cheat-sheet with the most common commands for:
- building and testing locally
- running `vpn-productd`
- calling the API
- enabling periodic system maintenance (sync/cleanup/backups) via systemd

For deeper operational docs, see `docs/DEPLOYMENT.md` and `docs/INCIDENT_RUNBOOK.md`.

## Local development (macOS/Linux)

From repo root:

```bash
make build
make test
make lint
make verify-quick
```

Full upstream+product test run (slow):

```bash
make verify
```

## Run `vpn-productd` locally (no systemd)

Minimal example:

```bash
export VPN_PRODUCT_API_TOKEN='change-me'
export VPN_PRODUCT_LISTEN='127.0.0.1:8080'
export VPN_PRODUCT_DATA_DIR='./var/vpn-product'
export VPN_PRODUCT_PUBLIC_BASE_URL='https://example.com'   # optional, used to build subscription URLs

mkdir -p "${VPN_PRODUCT_DATA_DIR}"
./vpn-productd --listen "${VPN_PRODUCT_LISTEN}" --data-dir "${VPN_PRODUCT_DATA_DIR}"
```

Env reference:
- `deploy/env/vpn-productd.env.example`

## API quick calls

```bash
export BASE_URL='http://127.0.0.1:8080'
export TOKEN='change-me'

curl -sS -H "Authorization: Bearer ${TOKEN}" "${BASE_URL}/v1/status"
curl -sS -H "Authorization: Bearer ${TOKEN}" "${BASE_URL}/v1/health"
curl -sS -H "Authorization: Bearer ${TOKEN}" "${BASE_URL}/v1/diagnostics/snapshot"
```

Issue a subscription link (30 days):

```bash
curl -sS -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data '{"userId":"tg_12345","profileIds":["xui-test-vpn"],"name":"VPN","source":"cli"}' \
  "${BASE_URL}/v1/issue/link"
```

Cleanup (revoke stale links + delete old rows):

```bash
curl -sS -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data '{"retentionDays":30,"staleDays":45}' \
  "${BASE_URL}/v1/internal/cleanup"
```

API reference:
- `docs/API.md`

## Staging deployment (recommended)

Canonical staging script:

```bash
cd /opt/vpn-product/src
sudo bash deploy/scripts/deploy_staging.sh
sudoedit /etc/vpn-product/vpn-productd.env
sudo systemctl restart vpn-productd
sudo bash deploy/scripts/smoke_staging.sh
```

See:
- `docs/DEPLOYMENT.md`

## systemd units (server)

All unit files live in `deploy/systemd/` and are installed by the deploy scripts.

### Core service

```bash
sudo systemctl status vpn-productd --no-pager
sudo journalctl -u vpn-productd -n 200 --no-pager
sudo systemctl restart vpn-productd
```

### Cleanup timer (DB maintenance)

Units:
- `deploy/systemd/vpn-product-cleanup.service`
- `deploy/systemd/vpn-product-cleanup.timer`

Env file example (you create it on the server):

```bash
sudoedit /etc/vpn-product/cleanup.env
```

Minimal content:

```bash
VPN_PRODUCT_API_TOKEN=change-me
VPN_PRODUCT_BASE_URL=http://127.0.0.1:8080
VPN_PRODUCT_CLEANUP_RETENTION_DAYS=30
VPN_PRODUCT_CLEANUP_STALE_DAYS=45
```

Enable + run:

```bash
sudo systemctl enable --now vpn-product-cleanup.timer
sudo systemctl start vpn-product-cleanup.service
sudo systemctl status vpn-product-cleanup.timer --no-pager
```

What it does:
- revokes active subscriptions whose last use is older than `staleDays` (default 45) so the link becomes invalid
- deletes old `subscription_issues` and long-revoked subscriptions using `retentionDays`

### Backups

Units:
- `deploy/systemd/vpn-product-backup.service`
- `deploy/systemd/vpn-product-backup.timer`

Manual run:

```bash
sudo systemctl start vpn-product-backup.service
sudo systemctl status vpn-product-backup.service --no-pager
```

Encryption (optional):

```bash
sudo systemctl edit vpn-product-backup.service
```

Add:

```ini
[Service]
Environment=BACKUP_ENCRYPT_KEY=change-me-very-strong
```

Notes:
- When `BACKUP_ENCRYPT_KEY` is set, archives are `vpn-migration-*.tar.gz.gpg`.
- Retention in `backup-server-state.sh` removes both plain and `.gpg` archives and their `.sha256`.

### 3x-ui sync / other timers

Common helpers (see `docs/DEPLOYMENT.md` for details):

```bash
sudo systemctl list-timers --all | grep vpn-product || true
sudo systemctl status vpn-product-sync-xui.timer --no-pager || true
sudo systemctl status vpn-product-sync-usage.timer --no-pager || true
sudo systemctl status vpn-geodata-update.timer --no-pager || true
```

## Telegram bot (optional)

Bot lives in `bot/` (aiogram 3). Typical run (example):

```bash
cd bot
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

# create bot/.env from .env.example, then:
python -m vpn_bot
```

The legacy `python-telegram-bot` tree is preserved under `archive/telegram-bot-legacy/`.

For the Mini App, set the **Menu Button / Mini App URL** in @BotFather to the **same HTTPS origin** as `WEBAPP_URL` in `telegram-miniapp/.env`. The Node process is normally **API + static only** (no second bot polling unless `START_TELEGRAM_BOT_POLLING=1`). Extend `bot/` if you want an inline **WebApp** keyboard button.

### Mini App (`telegram-miniapp/`)

```bash
cd telegram-miniapp
npm install
cp .env.example .env
# BOT_TOKEN (same bot as Python), VPN_API_URL, VPN_ADMIN_TOKEN (= VPN_PRODUCT_API_TOKEN), WEBAPP_URL=https://...
npm start
```

Production: install `deploy/systemd/vpn-tg-miniapp.service`, put env in `/etc/vpn-product/vpn-tg-miniapp.env`, reverse-proxy HTTPS (Caddy) to `PORT`. On `vpn-productd` set `VPN_PRODUCT_PUBLIC_BASE_URL` and `VPN_PRODUCT_SUBSCRIPTION_TOKEN_KEY` so `/admin/user/tg_…/status` can return `subUrl` for new or rotated subscriptions.

