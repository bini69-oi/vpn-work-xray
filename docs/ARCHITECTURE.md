# Architecture

## Components

```
┌──────────────┐     ┌──────────────────┐     ┌────────────┐
│  Telegram    │────▶│   vpn-productd   │────▶│   3x-ui    │
│     Bot      │     │   (Go API)       │     │  (панель)  │
└──────────────┘     └────────┬─────────┘     └─────┬──────┘
                              │                     │
                     ┌────────▼─────────┐     ┌─────▼──────┐
                     │   product.db     │     │  Xray-core │
                     │   (SQLite)       │     │  (трафик)  │
                     └──────────────────┘     └────────────┘
```

| Component | Path / binary | Role |
|-----------|----------------|------|
| VPN Product API | `cmd/vpn-productd`, package `internal/api` | REST API: profiles, subscriptions, routing, health, integration with 3x-ui |
| Business logic | `internal/profile`, `internal/subscription`, `internal/delivery`, … | Domain services and SQLite via `internal/storage/sqlite` |
| Client config generation | `internal/configgen` | Builds per-profile JSON for local Xray runtime |
| Routing & WARP | `internal/routing` | Preset rules (RU + WARP), geo helpers, WARP outbound snippets |
| Xray Core | `app/`, `common/`, `core/`, `features/`, `infra/`, `proxy/`, `transport/`, `main/` | Upstream [Xray-core](https://github.com/XTLS/Xray-core) sources; same module path `github.com/xtls/xray-core/...` |

Исходники ядра **не** вынесены в подкаталог `xray/`: при текущем `go.mod` (`module github.com/xtls/xray-core`) перенос ломал бы импорты.

## Data flow: new subscription

1. Admin или бот вызывает API выдачи подписки (`/admin/issue/link` или аналог).
2. Запись в `product.db` (подписка, токен, профили).
3. При необходимости — применение в 3x-ui (`client_traffics`, лимиты).
4. Клиент получает subscription URL; Happ/клиент тянет конфиг.

## Data flow: VPN client traffic

1. Пользователь подключается к inbound на сервере (конфигурация 3x-ui / Xray).
2. Трафик обрабатывается **xray-core** на хосте, не `vpn-productd`.
3. `vpn-productd` управляет учётом, синхронизацией и выдачей ссылок, а не шифрованием пользовательского трафика в этом контуре.

## Sync with 3x-ui

- Скрипты и эндпоинты (`sync_xui_to_product.sh`, `apply-to-3xui`, heartbeat) согласуют пользователей и лимиты между `product.db` и `x-ui.db`.
- Подробности деплоя: [DEPLOYMENT.md](DEPLOYMENT.md).

## Routing and WARP

- Пресет `VPN_PRODUCT_ROUTING_PRESET=ru_warp` в `internal/configgen` добавляет outbound `warp` и правила (direct RU, WARP для списка доменов).
- Ключи WARP: `WARP_MODE=wireguard|socks`, файл `warp.env` или переменные окружения.
- См. также [API.md](API.md) (раздел Routing).
