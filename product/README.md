# Product Layer Overview

`product/` provides a production-oriented VPN control layer on top of the existing Xray-core engine.

## Package layout

- `product/domain`: canonical product models and connection state types.
- `product/profile`: profile management service and validation.
- `product/storage/sqlite`: SQLite persistence for profiles and runtime state.
- `product/configgen`: profile-to-Xray config compiler and generated artifact writer.
- `product/connection`: connection orchestrator, runtime apply, reconnect/fallback flow.
- `product/reconnect`: retry strategy with jitter and bounded backoff.
- `product/health`: health probe contract.
- `product/diagnostics`: runtime snapshot aggregation.
- `product/logging`: product-level structured log wrapper.
- `product/api`: versioned local HTTP API (`/v1/*`) for CLI and future panels.
- `product/platform`: OS-specific hooks for system proxy and TUN capability checks.
- `product/integration/webhook`: stub event publisher for future external integrations.

## Upstream compatibility guardrails

- Core protocol/runtime packages (`core`, `proxy`, `transport`, `main/distro/all`) stay unchanged.
- Product behavior is implemented via new packages and dedicated binaries.
- Config generation targets Xray-compatible JSON without forking parser logic.

## Binaries

- `cmd/vpn-productd`: daemon that wires storage, orchestration, diagnostics, and local API.
- `cmd/vpn-productctl`: CLI client for `status`, `profiles`, `connect`, `disconnect`, `diagnostics`.
