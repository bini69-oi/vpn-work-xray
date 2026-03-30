# VPN Incident Runbook

This runbook is for fast incident response on the current single-server setup.

Server assumptions:
- `vpn-productd` local API: `127.0.0.1:8080`
- `x-ui` panel local: `127.0.0.1:13554`
- `xray` traffic listener: `:8443` (managed by `x-ui`)
- `caddy` public HTTPS entrypoint: `:443`

Useful paths:
- `/etc/vpn-product/vpn-productd.env`
- `/var/log/vpn-product/vpn-productd.log`
- `/etc/x-ui/x-ui.db`
- `/usr/local/x-ui/bin/config.json`

---

## 0) First 60 seconds (always)

```bash
systemctl is-active vpn-productd x-ui caddy
ss -lntup | sed -n '1,200p'
systemctl --failed --no-pager
```

If any core service is not `active`, jump to the matching section below.

---

## 1) Panel is down (`x-ui` unavailable)

Symptoms:
- Panel URL not opening
- `systemctl is-active x-ui` is `inactive` or `failed`

Actions:

```bash
systemctl restart x-ui
systemctl is-active x-ui
journalctl -u x-ui -n 120 --no-pager
ss -lntup | sed -n '/:13554/p;/:8443/p'
```

If still failing:
- Check if `8443` is occupied (see section 2).
- Verify DB access:

```bash
ls -ld /etc/x-ui
ls -l /etc/x-ui/x-ui.db
```

Expected:
- `x-ui` is `active`
- `:13554` and `:8443` are listening

---

## 2) Port `8443` is already in use

Symptoms in logs:
- `bind: address already in use` for `0.0.0.0:8443`

Actions:

```bash
ss -lntup | sed -n '/:8443/p'
ps -ef | sed -n '/xray/p'
```

If conflicting system `xray` service exists:

```bash
systemctl stop xray || true
systemctl disable xray || true
systemctl mask xray || true
systemctl restart x-ui
systemctl is-active x-ui
ss -lntup | sed -n '/:8443/p'
```

Expected:
- `x-ui` is `active`
- exactly one expected xray process owns `:8443`

---

## 3) Subscription link returns `404`

Symptoms:
- `{"error":"subscription not found","code":"VPN_SUBS_404"}`

Fast diagnostics:

```bash
API_TOKEN="$(sed -n 's/^VPN_PRODUCT_API_TOKEN=//p' /etc/vpn-product/vpn-productd.env | tr -d '\r\n')"
curl -s -o /tmp/sub_test.txt -w "%{http_code}\n" "https://<your-domain>/public/subscriptions/<token>"
curl -s -H "Authorization: Bearer ${API_TOKEN}" "http://127.0.0.1:8080/v1/issue/history?userId=<tg_user_id>&limit=5"
```

If user needs immediate restore:
1) Re-issue link:

```bash
curl -s -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  -X POST "http://127.0.0.1:8080/v1/issue/link" \
  -d '{"userId":"<tg_user_id>","name":"VPN","source":"incident-reissue"}'
```

2) Apply to 3x-ui:

```bash
curl -s -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  -X POST "http://127.0.0.1:8080/v1/issue/apply-to-3xui" \
  -d '{"userId":"<tg_user_id>","subscriptionId":"<sub-id-from-previous-step>"}'
```

Expected:
- New link returns `200`
- User has row in `client_traffics` for email `tg_user_id`

---

## 4) Renew/block flow is inconsistent

Use one-operation lifecycle endpoint only:

Renew:

```bash
curl -s -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  -X POST "http://127.0.0.1:8080/v1/subscriptions/lifecycle" \
  -d '{"userId":"<tg_user_id>","action":"renew","days":30}'
```

Block:

```bash
curl -s -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  -X POST "http://127.0.0.1:8080/v1/subscriptions/lifecycle" \
  -d '{"userId":"<tg_user_id>","action":"block"}'
```

Verify in `x-ui`:

```bash
python3 - <<'PY'
import sqlite3
con=sqlite3.connect('/etc/x-ui/x-ui.db')
cur=con.cursor()
print(cur.execute("select email,enable,total,expiry_time from client_traffics where email=? order by id desc limit 1", ('<tg_user_id>',)).fetchone())
PY
```

---

## 5) Rollback (safe and quick)

List available backups:

```bash
ls -1 /opt/vpn-product/rollback
```

Restore previous binaries:

```bash
cp /opt/vpn-product/rollback/vpn-productd.<stamp>.bak /usr/local/bin/vpn-productd
cp /opt/vpn-product/rollback/vpn-productctl.<stamp>.bak /usr/local/bin/vpn-productctl
chmod 0755 /usr/local/bin/vpn-productd /usr/local/bin/vpn-productctl
systemctl restart vpn-productd
```

Validate:

```bash
bash /opt/vpn-product/src/deploy/scripts/smoke_staging.sh
```

Notes:
- Rollback does not wipe `/var/lib/vpn-product` data.
- Do not remove `/etc/x-ui/x-ui.db` unless you have a tested restore.

---

## 6) Incident exit criteria

Incident is resolved only if all checks pass:

```bash
systemctl is-active vpn-productd x-ui caddy
bash /opt/vpn-product/src/deploy/scripts/smoke_staging.sh
curl -s -o /dev/null -w "%{http_code}\n" "https://<your-domain>/public/subscriptions/<known-good-token>"
```

Expected:
- services: `active`
- smoke script: `Smoke checks passed`
- known subscription: `200`
