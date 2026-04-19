# VPN Shield — Telegram-бот (aiogram 3, Remnawave-only)

Премиальный UX, Reply-меню, оплата (ручная / крипта / Telegram Payments / СБП), рефералы, админка (health Remnawave + рассылка).

Исходники сгруппированы в пакет **`vpn_bot/`**: `config/`, `database/`, `handlers/`, `keyboards/`, `middlewares/`, `services/`, `utils/`; сборка приложения — `vpn_bot/app.py`, запуск модулем `python -m vpn_bot` (тонкий `main.py` в корне каталога для совместимости).

## Связь с Remnawave Panel

Бот ходит в Panel REST API (префикс `/api`):

| Действие | Метод |
|----------|------|
| Получить пользователя по Telegram ID | `GET /api/users/by-telegram-id/{id}` |
| Создать пользователя | `POST /api/users` |
| Продлить подписку (`expireAt`) | `PATCH /api/users` |
| Подробности подписки | `GET /api/users/{uuid}` |
| Health-check панели | `GET /api/system/health` |

Реализация — `vpn_bot/services/remnawave_client.py`, контракт — `Protocol VPNBackend` в `vpn_bot/services/api_client.py`.

QR-коды бот **не генерирует** — только подписка/Happ-ссылка.

### Конфигурация

Все переменные — в `.env.example` рядом. Минимум:

```ini
BOT_TOKEN=...
ADMIN_IDS=123456789

REMNAWAVE_PANEL_URL=https://panel.example.com
REMNAWAVE_API_TOKEN=...
REMNAWAVE_INTERNAL_SQUAD_UUIDS=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
# REMNAWAVE_CADDY_TOKEN=...      # если панель за Caddy с X-Api-Key
```

## Запуск

Из корня репозитория:

```bash
make bot-venv     # создаёт .venv с runtime + dev-зависимостями
make bot          # запуск бота
make bot-test     # pytest (быстро)
make bot-cov      # pytest + coverage gate (>= 80%)
make bot-lint     # ruff
make bot-typecheck  # mypy
```

Или напрямую:

```bash
cd apps/vpn-telegram-bot
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt -r requirements-dev.txt
cp .env.example .env   # заполнить BOT_TOKEN, REMNAWAVE_*, ADMIN_IDS
python -m vpn_bot
```

## Логи

- **Локально** (`python -m vpn_bot`): stdout, уровень `INFO`.
- **systemd**: `journalctl -u <имя-сервиса> -f`.

## Docker

```bash
docker build -t vpn-bot ./apps/vpn-telegram-bot
docker run --rm --env-file .env vpn-bot
```

## Тесты

```bash
make bot-test    # быстро
make bot-cov     # с покрытием (порог 80%)
```

Покрыто:

- `RemnawaveApiClient` — wire format каждого вызова, обработка ошибок, заголовки (включая `X-Api-Key` для Caddy и `x-forwarded-*` для http-панели).
- `Settings` — алиасы legacy-переменных, парсинг squads, `api_configured()`.
- Backend factory `_build_backend_client` — все ветки.
- Subscription helpers — `vpn_user_id`, `delivery_profile_id`, `fetch_subscription_bundle`, `_pick_happ_import_link`, `_happ_add_url`.
- Monitoring formatter.

## Старая реализация

Предыдущий бот на `python-telegram-bot` — `archive/telegram-bot-legacy/` (не запускается из корня). Mini App, который тоже ходил в `vpn-productd`, — `archive/telegram-miniapp/`.
