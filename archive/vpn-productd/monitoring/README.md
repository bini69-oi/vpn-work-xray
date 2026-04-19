# Monitoring bootstrap

This folder contains baseline monitoring artifacts for vpn-product:

- `vpn-dashboard.json` — Grafana dashboard
- `alerts.yml` — Prometheus alert rules

## Key signals

- Issue-link pipeline: `vpn_subscription_issue_total`
- Apply-to-3x-ui: `vpn_apply_3xui_total`, `vpn_apply_3xui_latency_seconds`
- API failures: `vpn_api_5xx_total`
- Sync lag: `vpn_sync_lag_seconds`, `vpn_sync_last_success_unix`
