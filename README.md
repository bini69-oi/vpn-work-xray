<div align="center">

# VPN Product

**VPN-сервис на базе Xray-core с двумя сменными бэкендами выдачи: `vpn-productd` (legacy) и Remnawave Panel.**

Управление подписками • Telegram-бот и Mini App • Маршрутизация и WARP • Happ-клиент

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![Python](https://img.shields.io/badge/Python-3.12%20|%203.13-3776AB?style=flat-square&logo=python&logoColor=white)](https://python.org)
[![Xray](https://img.shields.io/badge/Xray--core-fork-blue?style=flat-square)](https://github.com/XTLS/Xray-core)
[![License](https://img.shields.io/badge/License-MPL--2.0-green?style=flat-square)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/bini69-oi/vpn-work-xray/ci.yml?style=flat-square&label=CI)](https://github.com/bini69-oi/vpn-work-xray/actions)

</div>

> 🚧 **Идёт миграция на Remnawave** (`Panel + Node`). Старый стек `vpn-productd` + 3x-ui ещё в работе — оба бэкенда переключаются одной переменной `VPN_BACKEND`. Прогресс и план: [`MIGRATION_TO_REMNAWAVE.md`](MIGRATION_TO_REMNAWAVE.md).

---

## Что это

VPN Product — слой вокруг [Xray-core](https://github.com/XTLS/Xray-core):

- **Go-сервис `vpn-productd`** (legacy backend): REST API, SQLite, интеграция с [3x-ui](https://github.com/MHSanaei/3x-ui), маршрутизация (RU + WARP).
- **Remnawave Panel + Node** (новый backend): официальный Docker-стек, разворачивается из [`deploy/remnawave/`](deploy/remnawave/README.md).
- **Telegram-клиенты в [`apps/`](apps/README.md)**:
  - [`vpn-telegram-bot/`](apps/vpn-telegram-bot/README.md) — Python (aiogram 3), **умеет работать с обоими бэкендами**.
  - [`telegram-miniapp/`](apps/telegram-miniapp/README.md) — Mini App: Node (Express) + статика.

Исходники ядра Xray лежат в корне (`app/`, `common/`, `core/`, `proxy/`, `transport/`, …) с модулем Go `github.com/xtls/xray-core` — их **не выносят** в подпапку, чтобы не ломать импорты. Код продукта — в [`internal/`](internal/README.md).

### Ключевые фичи

| | |
|---|---|
| 🔁 **Сменный бэкенд** | `VPN_BACKEND=productd` либо `remnawave` — без перекомпиляции; единый протокол `VPNBackend` в боте |
| 🌐 **Протоколы Xray** | VLESS/REALITY, VMess, Trojan, Shadowsocks, WireGuard, … |
| 📦 **Подписки** | REST API, лимиты, токены, выдача `subscription` URL |
| 🧭 **Маршрутизация** | Пресет `ru_warp` (direct RU + WARP outbound), geo-данные |
| 📱 **Happ** | Заголовок `hide-settings: 1` скрывает «JSON-стрелку» в клиенте; `support-url`, `profile-update-interval`, `announce` |
| 🛠 **Деплой** | systemd, Docker Compose для Remnawave, автобэкап Postgres, Caddy + автоматический SSL |
| ✅ **Тесты** | Go: модули `internal/*` + интеграционные; Python: 102 pytest-теста на бот; CI на push/PR |

---

## Архитектура (текущее состояние)

```
                ┌──────────────────────────────┐
                │     apps/  (Telegram)        │
                │  ┌─────────────┐  ┌────────┐ │
                │  │ aiogram bot │  │MiniApp │ │
                │  └──────┬──────┘  └───┬────┘ │
                └─────────┼─────────────┼──────┘
                          │ VPNBackend  │ HTTPS
            ┌─────────────┴──┐      ┌───┴──────────────────┐
            │ VPN_BACKEND=   │      │   VPN_BACKEND=       │
            │   productd     │      │     remnawave        │
            ▼                ▼      ▼                       ▼
   ┌──────────────────┐         ┌──────────────────────────────┐
   │  vpn-productd    │         │   Remnawave Panel  (Docker)  │
   │  (Go REST API)   │         │  ┌──────────┐  ┌──────────┐  │
   │  /v1/issue/...   │         │  │ Postgres │  │  Caddy   │  │
   │  /v1/subs/...    │         │  └──────────┘  └────┬─────┘  │
   └────────┬─────────┘         └────────────┬────────┴────────┘
            │ SQLite                         │ Node API
            ▼                                ▼
   ┌──────────────┐                ┌──────────────────┐
   │  3x-ui DB    │                │  Remnawave Node  │
   │  + Xray-core │                │  + Xray-core     │
   └──────────────┘                └──────────────────┘
```

Подробнее: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md). Деплой Remnawave: [`deploy/remnawave/README.md`](deploy/remnawave/README.md).

---

## Быстрый старт

### Сборка ядра и проверки

Требования: Go 1.26+, `golangci-lint`, Python 3.12/3.13.

```bash
git clone https://github.com/bini69-oi/vpn-work-xray.git
cd vpn-work-xray

make build          # vpn-productd, vpn-productctl
make verify-quick   # go test internal/* + golangci-lint + secret-scan
make verify         # полный прогон + покрытие (>= 80%); ~5 минут
```

### Telegram-бот (один из двух бэкендов)

```bash
make bot-venv       # создаёт apps/vpn-telegram-bot/.venv с runtime + dev-зависимостями
make bot-test       # 102 pytest-теста (Remnawave + productd клиенты, Settings, handlers)
make bot            # запуск
```

`apps/vpn-telegram-bot/.env` (пример — `.env.example`):

```ini
BOT_TOKEN=...
ADMIN_IDS=123456789

# legacy
VPN_BACKEND=productd
VPN_API_URL=http://127.0.0.1:8080
VPN_API_TOKEN=...

# или Remnawave
# VPN_BACKEND=remnawave
# REMNAWAVE_PANEL_URL=https://panel.example.com
# REMNAWAVE_API_TOKEN=...
# REMNAWAVE_INTERNAL_SQUAD_UUIDS=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
# REMNAWAVE_CADDY_TOKEN=...   # если панель за Caddy с X-Api-Key
```

### Mini App

```bash
cd apps/telegram-miniapp
npm install
cp .env.example .env       # заполни WEBAPP_URL, VPN_API_URL, VPN_ADMIN_TOKEN
npm start
```

Все остальные команды (systemd, прод, бэкапы): [`docs/COMMANDS.md`](docs/COMMANDS.md).

### Remnawave Panel + Node

На сервере панели:

```bash
sudo bash deploy/remnawave/scripts/install_panel.sh
# затем отредактировать /opt/remnawave/.env (PANEL_DOMAIN, JWT_*, SUB_PUBLIC_DOMAIN)
```

На сервере ноды:

```bash
sudo bash deploy/remnawave/scripts/install_node.sh
# отредактировать /opt/remnanode/.env (NODE_PORT, SECRET_KEY из UI)
```

Полный гайд + troubleshooting: [`deploy/remnawave/README.md`](deploy/remnawave/README.md).

---

## Конфигурация

### `vpn-productd` (Go-бэкенд)

| Переменная | Описание |
|------------|----------|
| `VPN_PRODUCT_API_TOKEN` | Bearer-токен для `/v1/` и `/api/v1/` |
| `VPN_PRODUCT_LISTEN` | Адрес прослушивания API (например `127.0.0.1:8080`) |
| `VPN_PRODUCT_DATA_DIR` | Каталог данных и `product.db` |
| `VPN_PRODUCT_ROUTING_PRESET` | Пусто или `ru_warp` |
| `VPN_PRODUCT_HAPP_HIDE_SETTINGS` | Скрыть «JSON-стрелку» в Happ (по умолчанию `1`) |
| `VPN_PRODUCT_HAPP_SUPPORT_URL` | URL поддержки в Happ |
| `VPN_PRODUCT_HAPP_UPDATE_INTERVAL_HOURS` | Период автообновления подписки в Happ |
| `WARP_MODE` | `wireguard` или `socks` |

Полный список — [`deploy/env/vpn-productd.env.example`](deploy/env/vpn-productd.env.example).

### Бот

`apps/vpn-telegram-bot/.env.example` — все переменные с комментариями.

### Remnawave Panel

`/opt/remnawave/.env` — формируется из [`deploy/remnawave/panel/.env.example`](deploy/remnawave/panel/.env.example).

---

## API

REST `vpn-productd`: [`docs/API.md`](docs/API.md).

Remnawave Panel REST: использует пути `/api/users`, `/api/users/by-telegram-id/{id}`, `/api/subscriptions`, `/api/system/health` — реализованы в `apps/vpn-telegram-bot/vpn_bot/services/remnawave_client.py` (контракт — Protocol `VPNBackend` в `services/api_client.py`). Официальная дока: <https://docs.rw>.

---

## Структура репозитория

| Путь | Назначение |
|------|------------|
| [`apps/`](apps/README.md) | Telegram-бот ([README](apps/vpn-telegram-bot/README.md)) и Mini App ([README](apps/telegram-miniapp/README.md)) |
| `cmd/vpn-productd`, `cmd/vpn-productctl` | Точки входа Go-бэкенда |
| [`internal/`](internal/README.md) | API, конфигген, профили, подписки, SQLite, routing |
| `app/`, `common/`, `core/`, `features/`, `infra/`, `proxy/`, `transport/`, `main/` | Исходники Xray-core |
| `tests/` | Тесты Go (включая `tests/integration/`) |
| `apps/vpn-telegram-bot/tests/` | Pytest для бота (102 теста) |
| [`deploy/remnawave/`](deploy/remnawave/README.md) | Docker Compose + scripts для Remnawave Panel/Node |
| `deploy/{systemd,scripts,env,caddy,grafana,prometheus.yml,monitoring}` | Деплой `vpn-productd` |
| `docs/` | API, деплой, runbooks, архитектура |
| `configs/` | Примеры конфигов Xray |
| `scripts/` | Сборка тестовых ассетов, `secret_scan.py` |
| [`archive/`](archive/README.md) | Снятый с эксплуатации код (legacy bot, ранние эксперименты) |

---

## Документация

| Документ | Описание |
|----------|----------|
| [`MIGRATION_TO_REMNAWAVE.md`](MIGRATION_TO_REMNAWAVE.md) | План и статус миграции |
| [`docs/API.md`](docs/API.md) | REST API `vpn-productd` |
| [`docs/COMMANDS.md`](docs/COMMANDS.md) | Команды: сборка, бот, Mini App, systemd |
| [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) | Деплой и эксплуатация |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Архитектура (legacy-стек) |
| [`docs/DR_RUNBOOK.md`](docs/DR_RUNBOOK.md) | Disaster recovery |
| [`docs/SLO.md`](docs/SLO.md) | SLO / baseline |
| [`docs/INCIDENT_RUNBOOK.md`](docs/INCIDENT_RUNBOOK.md) | Инциденты |
| [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) | Вклад в проект |
| [`deploy/remnawave/README.md`](deploy/remnawave/README.md) | Установка Remnawave Panel + Node |

---

## Roadmap (что ещё надо сделать)

> Чтобы закрыть миграцию и довести продукт до полноценного запуска на Remnawave.

### Срочно

- [ ] **Починить `test-lint` job в CI.** Падает на `make verify` ещё до моих изменений (см. историю). Локально пайплайн зелёный, на Ubuntu CI — red. Нужен анализ логов и фикс шага «Run full verify pipeline».
- [ ] **Mini App → Remnawave.** Сейчас [`apps/telegram-miniapp/`](apps/telegram-miniapp/README.md) умеет только `vpn-productd`. Нужно перевести на тот же контракт `VPNBackend`, что и aiogram-бот, либо ходить напрямую в Remnawave API.
- [ ] **Реальный Remnawave Provider ID для Happ.** Заголовок `hide-settings: 1` уже шлётся, но скрытие JSON-стрелки активируется только для подписок, привязанных к зарегистрированному Provider ID на happ.su. Нужно зарегистрироваться и привязать домен подписки.

### Миграция данных (этап 2 из `MIGRATION_TO_REMNAWAVE.md`)

- [ ] **Скрипт переноса пользователей** из `product.db` (SQLite) → Remnawave Panel: создание `User` через `POST /api/users` с сохранением `telegramId`, `expireAt`, оставшегося трафика и привязкой к нужному `internal squad`.
- [ ] **Сверка / dry-run** с отчётом «что переносится / что отброшено».
- [ ] **Cutover-runbook**: остановка `vpn-productd`, перенос, валидация, переключение `VPN_BACKEND=remnawave`, мониторинг.

### Платежи

- [ ] **Привязать оплату к Remnawave-подпискам.** Сейчас `_apply_paid_months` в [`apps/vpn-telegram-bot/vpn_bot/handlers/purchase.py`](apps/vpn-telegram-bot/vpn_bot/handlers/purchase.py) вызывает `lifecycle_renew(N*30)` — для Remnawave это `PATCH /api/users` c новым `expireAt`. Проверить:
  - идемпотентность повторной оплаты (что не накручиваются месяцы при двойном callback);
  - реферальный бонус (`REFERRAL_BONUS_DAYS`) — продление чужого `expireAt` через тот же путь;
  - возврат денег при ручном rejection админа (rollback продления).
- [ ] **Telegram Payments / СБП страница оплаты** (`SBP_PAY_URL`) — продакшен-ссылка вместо заглушки.

### Тесты и качество

- [x] Pytest для бота (Remnawave + productd клиенты, Settings, фабрика, handlers, monitoring) — **102 теста, в матрице Python 3.12 + 3.13**.
- [ ] **Интеграционный e2e-тест бота** против testcontainers с поднятой Remnawave Panel.
- [ ] **Coverage gate для Python-тестов** в CI (минимум, например, 85%).
- [ ] **Линтер Python** (`ruff` / `mypy`) в CI.
- [ ] **Coverage для бота-handlers**: сейчас покрыты сервисы и helpers, нужно добавить FSM-сценарии purchase/admin.

### Эксплуатация

- [ ] **Мониторинг Remnawave**: добавить Prometheus exporter / scrape config в [`deploy/monitoring/`](deploy/monitoring/README.md) и Grafana-дашборд (Panel API health, кол-во активных подписок, ошибки выдачи).
- [ ] **Алерты на просрочку бэкапа Postgres** (`vpn-backup.timer`) и на падение Remnawave Node.
- [ ] **Runbook для recovery Remnawave** (`docs/DR_RUNBOOK.md` сейчас описывает только legacy).

### Чистка после полного перехода

- [ ] Удалить `vpn-productd` и его 3x-ui интеграцию **только после** успешного cutover и backup-а данных.
- [ ] Перенести релевантную часть `internal/` в `archive/` либо в отдельный репо.
- [ ] Обновить `docs/ARCHITECTURE.md` — заменить legacy-диаграмму на Remnawave-only.

---

## Лицензия

[MPL-2.0](LICENSE). Проект основан на [Xray-core](https://github.com/XTLS/Xray-core).
