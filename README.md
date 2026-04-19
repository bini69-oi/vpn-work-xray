<div align="center">

# VPN Product — Remnawave + Telegram bot

**Коммерческий VPN-сервис:** Remnawave Panel на одном сервере, Remnawave Node на другом, Telegram-бот (aiogram 3) для продаж и подключения.

[![CI](https://img.shields.io/github/actions/workflow/status/bini69-oi/vpn-work-xray/ci.yml?style=flat-square&label=CI)](https://github.com/bini69-oi/vpn-work-xray/actions)
[![Python](https://img.shields.io/badge/Python-3.12%20|%203.13-3776AB?style=flat-square&logo=python&logoColor=white)](https://python.org)
[![Remnawave](https://img.shields.io/badge/Remnawave-Panel%20%2B%20Node-blue?style=flat-square)](https://docs.rw)
[![Coverage](https://img.shields.io/badge/coverage-%E2%89%A580%25-brightgreen?style=flat-square)](#тесты-и-качество)
[![License](https://img.shields.io/badge/License-MPL--2.0-green?style=flat-square)](LICENSE)

</div>

---

## Что это

- [**Remnawave Panel + Node**](deploy/remnawave/README.md) — официальный Docker-стек Remnawave (`https://docs.rw`). Панель ставится одной командой, нода — второй. Все конфиги и `install_*.sh` лежат в [`deploy/remnawave/`](deploy/remnawave/).
- [**Telegram-бот**](apps/vpn-telegram-bot/README.md) (aiogram 3, Python 3.12/3.13) — общается с Remnawave Panel по REST API (`/api/users`, `/api/system/health`, …). Управляет подписками, тарифами, оплатой (СБП/Telegram Payments), рефералами, рассылками.
- **Two-server setup by default**: бот и панель могут стоять где угодно (обычно бот на том же хосте, что и панель; нода отдельно).

> **История.** Раньше здесь жил Go-сервис `vpn-productd` + форк Xray-core + Mini App на Node. Всё это снято с эксплуатации и вынесено в [`archive/`](archive/README.md). Исходники Xray-core удалены, их можно подтянуть как зависимость у upstream по мере необходимости.

---

## Архитектура (two servers)

```
 ┌─────────────────────────────┐
 │  Сервер 1 — Remnawave Panel │
 │  ┌───────────────────────┐  │
 │  │ Postgres  Redis       │  │
 │  │ remnawave (REST API)  │  │        Telegram Bot API
 │  │ subscription-page     │  │◀─────────── aiogram 3 ───────────┐
 │  │ Caddy (HTTPS)         │  │                                  │
 │  └───────────────────────┘  │     /api/users/by-telegram-id    │
 └─────────────┬───────────────┘     /api/users  (POST/PATCH)     │
               │                     /api/system/health           │
               │ внутренняя сеть                                  │
               │ NODE_PORT только для IP панели                   │
 ┌─────────────┴───────────────┐                                  │
 │  Сервер 2 — Remnawave Node  │                                  │
 │  ┌───────────────────────┐  │                                  │
 │  │ xray-core             │  │                                  │
 │  │ remnawave-node        │  │                                  │
 │  └───────────────────────┘  │                                  │
 └─────────────────────────────┘                                  │
                                                                  │
                                                    Пользователь ─┘
                                       (Happ / другой VLESS-клиент)
```

Бот хранит только свои артефакты: SQLite (`users/payments/referrals/broadcasts`) — всё, что связано с подписками, живёт в Remnawave.

---

## Быстрый старт (2 сервера)

### 1. Сервер 1 — Remnawave Panel

```bash
git clone https://github.com/bini69-oi/vpn-work-xray.git
cd vpn-work-xray

sudo bash deploy/remnawave/scripts/install_panel.sh
sudoedit /opt/remnawave/.env     # PANEL_DOMAIN, SUB_PUBLIC_DOMAIN, JWT_AUTH_SECRET, JWT_API_TOKENS_SECRET
cd /opt/remnawave && docker compose up -d
```

DNS: `PANEL_DOMAIN` и `SUB_DOMAIN` должны резолвиться на IP панели, Caddy сам выпустит SSL.

Первый зарегистрированный в UI пользователь — **super-admin** (см. [`deploy/remnawave/README.md`](deploy/remnawave/README.md)).

### 2. Сервер 2 — Remnawave Node

В UI панели: `Management → Nodes → +` — получите `SECRET_KEY` и выберите `NODE_PORT`.

```bash
git clone https://github.com/bini69-oi/vpn-work-xray.git
cd vpn-work-xray
sudo bash deploy/remnawave/scripts/install_node.sh
sudoedit /opt/remnanode/.env     # NODE_PORT + SECRET_KEY из UI
cd /opt/remnanode && docker compose up -d
```

Фаервол: `NODE_PORT` открыт **только** для IP панели.

### 3. Telegram-бот (любой из двух серверов или третий)

```bash
cd apps/vpn-telegram-bot
cp .env.example .env
# заполнить: BOT_TOKEN, ADMIN_IDS, REMNAWAVE_PANEL_URL,
#            REMNAWAVE_API_TOKEN, REMNAWAVE_INTERNAL_SQUAD_UUIDS
cd ../..
make bot-venv        # python3 -m venv + requirements
make bot             # запуск aiogram-бота
```

Токен Remnawave API берётся в UI: `Settings → API tokens → +`. `INTERNAL_SQUAD_UUIDS` — UUID внутреннего сквада (профиля), куда добавляются новые пользователи при оплате.

Боевой запуск — через `systemd`/`docker`. Пример юнита и `docker-compose` для бота см. [`apps/vpn-telegram-bot/README.md`](apps/vpn-telegram-bot/README.md).

---

## Конфигурация бота

`apps/vpn-telegram-bot/.env` (подробности — в [`.env.example`](apps/vpn-telegram-bot/.env.example)):

```ini
# Telegram
BOT_TOKEN=123456:AA...
ADMIN_IDS=123456789
BOT_USERNAME=my_vpn_bot

# Remnawave Panel API
REMNAWAVE_PANEL_URL=https://panel.example.com
REMNAWAVE_API_TOKEN=<bearer из UI>
REMNAWAVE_INTERNAL_SQUAD_UUIDS=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
# если панель за Caddy с X-Api-Key:
# REMNAWAVE_CADDY_TOKEN=...

# Платежи
PAYMENT_STUB=1              # 1 = кнопка «Я оплатил» всегда зачисляет (тест)
PAYMENT_STUB_RESULT=ok      # ok | fail
PAYMENT_PROVIDER_TOKEN=     # Telegram Payments (опционально)
SBP_PAY_URL=                # ссылка на страницу СБП (опционально)

# Рефералка
REFERRAL_BONUS_DAYS=15
```

---

## Команды

| Цель | Команда |
|------|---------|
| Создать виртуальное окружение бота | `make bot-venv` |
| Запустить бота | `make bot` |
| Тесты (быстро) | `make bot-test` |
| Тесты + покрытие (≥ 80 %) | `make bot-cov` |
| Линтер (ruff) | `make bot-lint` |
| Статическая типизация (mypy) | `make bot-typecheck` |
| Полный прогон CI (secret-scan + lint + typecheck + cov) | `make verify` |
| Установить git-хуки | `bash scripts/install_git_hooks.sh` |
| Чистка артефактов | `make clean` |

Подробный справочник команд (установка Remnawave, бэкапы, отладка бота): [`docs/COMMANDS.md`](docs/COMMANDS.md).

---

## Структура репозитория

| Путь | Назначение |
|------|------------|
| [`apps/vpn-telegram-bot/`](apps/vpn-telegram-bot/README.md) | Python-бот (aiogram 3), единственный клиент к Remnawave Panel API |
| [`deploy/remnawave/`](deploy/remnawave/README.md) | Docker Compose + `install_*.sh` для Panel / Node + Caddy + systemd-бэкап |
| [`deploy/env/`](deploy/env/) | Пример `.env` для бота (productd-легаси удалён) |
| [`docs/`](docs/) | [COMMANDS.md](docs/COMMANDS.md), [CONTRIBUTING.md](docs/CONTRIBUTING.md) |
| [`scripts/`](scripts/) | `secret_scan.py`, git-хуки (`pre-commit`/`pre-push`) |
| [`archive/`](archive/README.md) | `vpn-productd/`, `telegram-bot-legacy/`, `telegram-miniapp/` — всё, что снято с эксплуатации |

---

## Тесты и качество

Бот покрыт **205 тестами** и проходит линтер + mypy + coverage-gate ≥ 80 %:

- `tests/test_remnawave_client.py` — unit-тесты REST-клиента (FakeSession, все пути).
- `tests/test_remnawave_integration.py` — **e2e против реального aiohttp-сервера**, эмулирующего Remnawave Panel: жизненный цикл подписки, заголовки, 404.
- `tests/test_handlers_purchase.py` — FSM покупки (plans → pay methods → СБП → admin confirm/reject, successful_payment).
- `tests/test_handlers_admin.py` — FSM рассылки, мониторинг (`mon_refresh`), права админа.
- `tests/test_handlers_misc.py` — `/start`, `🛡 Мой VPN`, Happ deep link, FAQ, рефералка.
- `tests/test_settings.py`, `test_formatting.py`, `test_referral_service.py`, `test_monitoring.py`, `test_subscription_service.py`, `test_middlewares.py` — сервисы и хелперы.

Запуск локально:

```bash
make bot-venv
make bot-cov        # pytest --cov=vpn_bot --cov-fail-under=80
```

CI (GitHub Actions, [`.github/workflows/ci.yml`](.github/workflows/ci.yml)):

1. `secret-scan` — ищет утечки токенов.
2. `bot-quality` — матрица Python 3.12 + 3.13: `ruff` → `mypy` → `pytest --cov-fail-under=80`.

---

## Git hooks

```bash
bash scripts/install_git_hooks.sh
```

- `pre-commit`: `secret-scan` + `ruff` + `mypy` (быстрые проверки).
- `pre-push`: `make verify` — secret-scan, ruff, mypy, pytest + coverage (если `.venv` бота установлен).

---

## Документация

| Документ | Что внутри |
|----------|------------|
| [`apps/vpn-telegram-bot/README.md`](apps/vpn-telegram-bot/README.md) | Архитектура бота, таблица вызовов Remnawave API, конфиг, запуск, тесты |
| [`deploy/remnawave/README.md`](deploy/remnawave/README.md) | Установка Panel + Node, DNS, SSL (Caddy), бэкапы, troubleshooting |
| [`docs/COMMANDS.md`](docs/COMMANDS.md) | Полная шпаргалка по командам |
| [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) | Процесс разработки |
| [`archive/README.md`](archive/README.md) | Что лежит в `archive/` и почему |

---

## Лицензия

[MPL-2.0](LICENSE).
