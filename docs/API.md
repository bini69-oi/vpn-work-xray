# VPN Product API

Base URL: `http://127.0.0.1:8080`  
Auth: `Authorization: Bearer <VPN_PRODUCT_API_TOKEN>` for all endpoints.

## Error format

```json
{
  "error": "human-readable message",
  "code": "VPN_API_METHOD_001"
}
```

## Endpoints

### `GET /v1/status`
- Description: Current runtime state.
- Response `200`: `{"status": {...runtimeStatus...}}`

### `POST /v1/connect`
- Description: Connect runtime using profile.
- Request:
```json
{"profileId":"secure-performance-default"}
```
- Response `200`: status object.
- Errors: `400` validation/connect errors.

### `POST /v1/disconnect`
- Description: Stop active runtime.
- Response `200`: status object.

### `GET /v1/profiles`
- Description: List profiles.
- Response `200`: `{"profiles":[...]}`

### `POST /v1/profiles/upsert`
- Description: Create/update profile.
- Request: full `domain.Profile` JSON.
- Response `200`: `{"profile":{...}}`

### `POST /v1/profiles/delete`
- Description: Delete profile.
- Request:
```json
{"profileId":"profile-id"}
```
- Response `200`: `{"deleted":true}`

### `GET /v1/account`
- Description: Account/subscription status.
- Optional query: `profileId`.
  - no `profileId` -> `unknown`
  - profile exists and `blocked=false` -> `active`
  - profile exists and `blocked=true` -> `expired`
- Response `200`:
```json
{"account":{"status":"active|expired|unknown"}}
```
- Response `404`: `VPN_ACCOUNT_404` (profile not found when `profileId` is provided)

### `GET /v1/diagnostics/snapshot`
- Description: Runtime + health snapshot.
- Response `200`: diagnostics payload.

### `GET /v1/health`
- Description: Deep self-check (DB/runtime/assets/network).
- Response `200` for healthy, `503` for unhealthy.

### `GET /v1/metrics`
- Description: Prometheus metrics.
- Response `200`: text exposition format.

### `GET /v1/delivery/links?profileId=<id>`
- Description: Generate client import links for all endpoints in profile.
- Response `200`:
```json
{
  "profileId":"secure-performance-default",
  "links":{
    "primary":"vless://...",
    "fallback-hy2":"h2://...",
    "fallback-wg":"wireguard://..."
  }
}
```
- Errors:
  - `400` `VPN_DELIVERY_001` when `profileId` is missing
  - `404` `VPN_DELIVERY_404` when profile not found

### `POST /v1/quota/set`
- Description: Set traffic limit for profile.
- Request:
```json
{"profileId":"p1","limitMb":1024}
```
- Response `200`: `{"ok":true}`

### `POST /v1/quota/add`
- Description: Add traffic counters manually.
- Request:
```json
{"profileId":"p1","uploadBytes":1024,"downloadBytes":2048}
```
- Response `200`: `{"ok":true}`

### `POST /v1/quota/block`
- Description: Toggle profile block flag.
- Request:
```json
{"profileId":"p1","blocked":true}
```
- Response `200`: `{"ok":true}`

### `GET /v1/stats/profiles`
- Description: Per-profile traffic/quota stats.
- Response `200`: `{"items":[...]}`

### `POST /v1/integration/3xui/users/upsert`
- Description: Upsert 3X-UI user mapping.
- Request: `domain.PanelUser`.
- Response `200`: `{"ok":true}`

### `POST /v1/internal/cleanup`
- Description: Delete old rows from `subscription_issues` and old revoked subscriptions from `subscriptions`.
- Auth: same as `/v1/*` (bearer token).
- Request (optional):
```json
{"retentionDays":30}
```
- Response `200`:
```json
{"ok":true,"deleted":123}
```

### `GET /v1/integration/3xui/users?panel=3x-ui`
- Description: List panel users.
- Response `200`: `{"items":[...]}`

### `POST /v1/issue/link`
- Description: Issue a personal subscription link for user (30 days) and auto-apply it to `3x-ui` for panel visibility.
- Request:
```json
{"userId":"tg_12345","profileIds":["xui-test-vpn"],"name":"TEST VPN","source":"telegram-miniapp"}
```
- Notes:
  - `profileIds` is optional; default is `["xui-test-vpn"]`.
  - `expiresAt` is set automatically to now + 30 days.
  - Per-user limits are applied automatically: `30 days + 1 TB` in `3x-ui` (`client_traffics`).
  - If the user already has a subscription (even expired or revoked), the server may **renew/reactivate** it instead of creating a new one.
    In that case the existing token is kept (no new URL is generated) and the response `url` can be empty.
- Response `200`:
```json
{"subscription": {...}, "url":"https://<host>/public/subscriptions/<token>", "days":30, "appliedTo3xui":true, "profileId":"user-tg-12345"}
```

### `GET /v1/issue/history?userId=<id>&limit=50`
- Description: Return issuance history for user (from DB `subscription_issues`).
- Response `200`:
```json
{"items":[...]}
```

### `POST /v1/issue/apply-to-3xui`
- Description: Bind issued subscription to a personal 3x-ui client (`email=tg_user_id`), set `1TB` + expiry from subscription, and rebind subscription to personal profile.
- Request:
```json
{"userId":"tg_12345","subscriptionId":"sub-...","baseProfileId":"xui-test-vpn"}
```
- Response `200`:
```json
{"ok":true,"subscriptionId":"sub-...","profileId":"user-tg-12345"}
```

### `POST /v1/subscriptions/lifecycle`
- Description: One-operation renew/block in both `vpn-product` and `3x-ui`.
- Request renew:
```json
{"userId":"tg_12345","action":"renew","days":30}
```
- Request block:
```json
{"userId":"tg_12345","action":"block"}
```
- Response `200`:
```json
{"ok":true,"action":"renew|block","subscriptionId":"sub-...","expiresAt":"..."}
```
 - Notes:
   - Renew keeps the same subscription token (clients do not need reconfiguration).
   - Renew works for active, expired and revoked subscriptions (reactivates when needed).

### Routing (Xray rules, geo data, WARP)

Same auth as `/v1/*`. Paths are also available under `/admin/`.

### `GET /api/v1/routing/rules`
- Description: Current custom routing JSON (if `VPN_PRODUCT_ROUTING_RULES_PATH` exists and is non-empty).
- Response `200`: `{"routing":{...}|null,"path":"..."}`

### `PUT /api/v1/routing/rules`
- Description: Replace custom routing (`domainStrategy`, `domainMatcher`, `rules`). When saved, configgen uses this instead of built-in presets.
- Request: `RoutingConfigFile` JSON with non-empty `rules`.

### `POST /api/v1/routing/reload`
- Description: Re-run connect for the active profile so a new JSON is generated and applied.
- Response `200`: `{"reloaded":true,"profileId":"..."}` or `reloaded:false` if idle.

### `GET /api/v1/routing/geodata/status`
- Description: File size and mtime for `geoip.dat` / `geosite.dat` in `VPN_PRODUCT_GEODATA_DIR` (default `/usr/local/x-ui/bin`).

### `POST /api/v1/routing/geodata/update`
- Description: Force-download geo files to `GeodataDir` and mirror to `VPN_PRODUCT_DATA_DIR/assets` when data dir is set.

### `GET /api/v1/routing/warp/status`
- Description: WARP mode (`WARP_MODE`), key presence, SOCKS reachability.

### `POST /api/v1/routing/warp/setup`
- Description: Runs `VPN_PRODUCT_SETUP_WARP_SCRIPT` (default `/opt/vpn-product/src/deploy/scripts/setup_warp.sh`) if present.

### `PUT /api/v1/routing/warp/domains`
- Description: Update `VPN_PRODUCT_WARP_DOMAINS_PATH` (`domains`, `geosite_tags`).

## Security notes

- `VPN_PRODUCT_API_TOKEN` protects `/v1/*`.
- Optional `VPN_PRODUCT_ADMIN_TOKEN` can protect `/admin/*` with a separate token.
- Optional 3x-ui integration env:
  - `VPN_PRODUCT_3XUI_DB_PATH=/etc/x-ui/x-ui.db`
  - `VPN_PRODUCT_3XUI_INBOUND_PORT=8443`
