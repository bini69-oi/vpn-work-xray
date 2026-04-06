<div align="center">

# VPN Product

**Высокопроизводительный VPN-сервис на базе Xray-core**

Управление подписками • Интеграция с 3x-ui • Telegram-бот • Маршрутизация и WARP

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![Xray](https://img.shields.io/badge/Xray--core-fork-blue?style=flat-square)](https://github.com/XTLS/Xray-core)
[![License](https://img.shields.io/badge/License-MPL--2.0-green?style=flat-square)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/bini69-oi/vpn-work-xray/ci.yml?style=flat-square&label=CI)](https://github.com/bini69-oi/vpn-work-xray/actions)

</div>

---

## Что это

VPN Product — слой вокруг [Xray-core](https://github.com/XTLS/Xray-core): API (`vpn-productd`), SQLite, интеграция с [3x-ui](https://github.com/MHSanaei/3x-ui), скрипты деплоя и опционально Telegram-бот.

Исходники самого ядра Xray лежат в корне репозитория (`app/`, `common/`, `core/`, `proxy/`, `transport/`, …) с тем же модулем Go `github.com/xtls/xray-core` — **мы не переносим их в подпапку `xray/`**, чтобы не ломать импорты. Код продукта — в **`internal/`**.

### Ключевые возможности

- **Протоколы** — наследуются от Xray (VLESS/REALITY, VMess, Trojan, Shadowsocks, WireGuard и др.)
- **Профили и подписки** — REST API, лимиты, выдача subscription links
- **3x-ui** — синхронизация пользователей и лимитов
- **Маршрутизация** — пресеты RU + WARP, geo-данные, правила в JSON ([docs/API.md](docs/API.md))
- **Деплой** — systemd, скрипты в `deploy/scripts/`, пример Caddy: [deploy/caddy/Caddyfile.example](deploy/caddy/Caddyfile.example)

---

## Архитектура

См. подробнее [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

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

**Поток данных (упрощённо):**

1. Клиент или бот обращается к API `vpn-productd`.
2. Состояние подписок и профилей хранится в SQLite (`VPN_PRODUCT_DATA_DIR`).
3. При необходимости выполняется синхронизация с 3x-ui.
4. Пользовательский VPN-трафик обрабатывает Xray на сервере (конфигурация панели), а не сам `vpn-productd`.

---

## Быстрый старт (разработка)

### Требования

- Go 1.26+ (см. `go.mod`)
- `golangci-lint` для `make lint`

### Сборка и проверки

```bash
git clone https://github.com/bini69-oi/vpn-work-xray.git
cd vpn-work-xray

make build          # бинарники vpn-productd, vpn-productctl в корне репозитория
make test
make lint
make verify-quick   # тесты product + линтер + secret-scan
make verify         # полный прогон включая все пакеты xray-core (долго)
```

Конфигурация для сервера: скопируйте [deploy/env/vpn-productd.env.example](deploy/env/vpn-productd.env.example) в `/etc/vpn-product/vpn-productd.env` и задайте токены.

---

## Конфигурация

Основные переменные (полный список — в [deploy/env/vpn-productd.env.example](deploy/env/vpn-productd.env.example) и [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)):

| Переменная | Описание |
|------------|----------|
| `VPN_PRODUCT_API_TOKEN` | Bearer-токен для `/v1/` и `/api/v1/` |
| `VPN_PRODUCT_LISTEN` | Адрес прослушивания API (например `127.0.0.1:8080`) |
| `VPN_PRODUCT_DATA_DIR` | Каталог данных и `product.db` |
| `VPN_PRODUCT_ROUTING_PRESET` | Пусто или `ru_warp` для пресета маршрутизации |
| `WARP_MODE` | `wireguard` или `socks` для outbound WARP |

Примеры JSON: [configs/](configs/).

---

## API

Документация: [docs/API.md](docs/API.md)

Публичные и админ-маршруты описаны там же (в т.ч. `/api/v1/routing/*`).

---

## Структура репозитория

| Путь | Назначение |
|------|------------|
| `cmd/vpn-productd`, `cmd/vpn-productctl` | Точки входа продукта |
| `internal/` | API, конфигген, профили, подписки, SQLite, routing, … |
| `app/`, `common/`, `core/`, `features/`, `infra/`, `proxy/`, `transport/`, `main/` | Исходники Xray-core (upstream layout) |
| `tests/integration/` | Интеграционные тесты сценариев (ранее `testing/`) |
| `deploy/` | systemd, скрипты, env-примеры, Caddy |
| `docs/` | API, деплой, runbooks |
| `configs/` | Примеры конфигов |
| `telegram-bot/` | Опциональный бот (Python) |

---

## Документация

| Документ | Описание |
|----------|----------|
| [docs/API.md](docs/API.md) | REST API |
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | Деплой и эксплуатация |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Архитектура |
| [docs/DR_RUNBOOK.md](docs/DR_RUNBOOK.md) | Disaster recovery |
| [docs/SLO.md](docs/SLO.md) | SLO / baseline |
| [docs/INCIDENT_RUNBOOK.md](docs/INCIDENT_RUNBOOK.md) | Инциденты |
| [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) | Вклад в проект |

---

## Лицензия

[MPL-2.0](LICENSE). Проект основан на [Xray-core](https://github.com/XTLS/Xray-core).
