# Архив: vpn-productd legacy stack

Этот каталог сохраняет полный snapshot старого backend-стека на момент cutover на Remnawave Panel.

## Содержимое

| Подкаталог | Что было |
|------------|----------|
| `cmd/` | Go-точки входа: `vpn-productd` (HTTP API), `vpn-productctl` (CLI) |
| `internal/` | Бизнес-логика: SQLite, configgen, lifecycle, sync с 3x-ui, routing, и т.д. |
| `systemd/` | Все unit/timer-файлы (`vpn-productd.service`, `vpn-product-backup.timer`, `vpn-tg-miniapp.service`, sync-таймеры, …) |
| `scripts/` | Helper-скрипты деплоя: backup/restore, cleanup, lifecycle daily, sync 3x-ui ↔ product, smoke-тесты, install-таймеры |
| `env/` | `vpn-productd.env.example`, `sync-xui.env.example`, `lifecycle-sync.env.example`, `alerts.env.example` |
| `caddy/` | `Caddyfile.example` (reverse proxy с X-Api-Key) |
| `monitoring/`, `grafana/`, `prometheus.yml`, `docker-compose.monitoring.yml`, `logrotate/` | Стек мониторинга для vpn-productd |

## Чем заменено

- **Backend**: Remnawave Panel + Node — см. `deploy/remnawave/`
- **Бот**: `apps/vpn-telegram-bot/` (aiogram 3, общается с Remnawave Panel REST)

## Восстановление

Чтобы снова собрать `vpn-productd`, нужно вернуть:

1. `cmd/` и `internal/` в корень репозитория.
2. `go.mod`, `go.sum`, `.golangci.yml`, `Dockerfile`, Makefile-цели — из git-истории до коммита cutover.
3. Исходники Xray-core форка (`app/`, `common/`, `core/`, …) — тоже из git-истории.

После этого `make build && make verify` снова работает как раньше.
