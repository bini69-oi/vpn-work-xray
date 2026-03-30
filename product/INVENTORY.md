# Xray Inventory And Extension Boundaries

This document fixes a baseline inventory for the VPN product layer.

## Core runtime that must stay upstream-compatible

- `core`: instance lifecycle, config loading, stable embedding API.
- `main`: CLI entrypoints and runtime bootstrap.
- `main/distro/all`: component registration via side-effect imports.
- `proxy` and `transport`: protocol and transport implementation details.
- `infra/conf`: config translation and merge semantics.

## Existing modules reused as product engine

- `app/proxyman`: runtime inbound and outbound management.
- `app/router`: routing rules, balancers, route selection.
- `app/dns`: DNS strategy and fallback handling.
- `app/stats`: counters and online stats.
- `app/observatory`: outbound health and availability signals.
- `app/commander`: gRPC control services used for runtime management.
- `app/log`: runtime logging implementation.

## Product-layer extension strategy

- Add a dedicated layer under `product/` for domain, state, orchestration, and diagnostics.
- Add separate binaries under `cmd/vpn-productd` and `cmd/vpn-productctl`.
- Keep generated runtime config and operational state outside upstream core packages.
- Prefer integration through existing Xray capabilities and API contracts.

## Protected and extension zones

- Protected (avoid direct edits): `core`, `main/distro/all`, `proxy`, `transport`.
- Extension zones: `cmd/vpn-productd`, `cmd/vpn-productctl`, and all `product/*` packages.
