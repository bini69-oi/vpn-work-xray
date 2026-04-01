# DR Drill Runbook

## Goal

Validate backup restore path and capture observed RTO/RPO.

## Inputs

- latest backup archive
- checksum file (`.sha256`)
- target host (staging or isolated restore node)

## Timeline markers

- `T0`: incident start (restore decision)
- `Tbackup`: timestamp of latest valid backup
- `Tservice`: all core services are `active`
- `Tbusiness`: end-to-end smoke is successful

## Procedure

1. Verify backup integrity:
   - `sha256sum -c <archive>.sha256`
2. Restore state:
   - `bash deploy/scripts/import_server_state.sh <archive>`
3. Service checks:
   - `systemctl is-active vpn-productd x-ui caddy`
4. Functional checks:
   - `bash deploy/scripts/smoke_staging.sh`
   - `bash deploy/scripts/smoke_issue_pipeline.sh`
5. Capture evidence:
   - command output
   - service logs
   - smoke results

## Calculations

- `RTO = Tbusiness - T0`
- `RPO = T0 - Tbackup`

## Drill report template

- Date:
- Operator:
- Environment:
- T0:
- Tbackup:
- Tservice:
- Tbusiness:
- RTO:
- RPO:
- Issues found:
- Follow-up actions:
