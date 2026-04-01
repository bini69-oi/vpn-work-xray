# SLO and Load Baseline

## SLI/SLO (v1)

- API availability (2xx/3xx share) for `/v1/*`: **>= 99.9%** over 30d.
- API latency for admin/v1 endpoints:
  - **p95 < 200ms**
  - **p99 < 500ms**
- Error rate (`5xx / all`): **< 0.5%** during steady load.

## Metrics source

- `vpn_api_response_time_seconds{method,path,status}`
- `vpn_api_5xx_total{method,path,status}`
- `vpn_subscription_issue_total{result}`
- `vpn_apply_3xui_total{result}`
- `vpn_apply_3xui_latency_seconds{result}`
- `vpn_sync_lag_seconds{sync_name}`
- `vpn_sync_last_success_unix{sync_name}`

## Reproducible load profile

Use `scripts/load_baseline.sh` against staging endpoint.

Scenarios:
- steady: fixed RPS for 10m
- spike: 3x RPS for 2m
- recovery: return to baseline RPS for 5m

## Baseline artifacts

- Current baseline: `benchmarks/baseline/current.json`
- Update rule: baseline can be updated only after:
  1. CI is green
  2. no open Sev1/Sev2 incidents
  3. load run attached in PR

## Runbook

1. Run load script and store output under `benchmarks/load/`.
2. Compare output with `benchmarks/baseline/current.json`.
3. Fail release if p95/p99/error-rate regressions violate SLO.
