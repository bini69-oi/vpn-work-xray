# VPN Shield — бот (aiogram 3)

Отдельный сервис в каталоге `bot/`: премиальный UX, Reply-меню, оплата (ручная / крипта / Telegram Payments), рефералы, админка (health + рассылка).

Исходники сгруппированы в пакет **`vpn_bot/`**: `config/`, `database/`, `handlers/`, `keyboards/`, `middlewares/`, `services/`, `utils/`; сборка приложения — в `vpn_bot/app.py`, запуск модулем `python -m vpn_bot` (в корне `bot/` остаётся тонкий `main.py` для совместимости).

## Связь с vpn-productd

Используются реальные пути: `/v1/issue/status`, `/v1/issue/link`, `/v1/subscriptions/lifecycle`, `/v1/subscriptions/{id}`, `/v1/delivery/links`, `/v1/stats/profiles`, `/v1/health`.

Переменные: `VPN_API_URL`, `VPN_API_TOKEN` (как `VPN_PRODUCT_API_TOKEN` на сервере).

**QR-коды не генерируются** (по требованию продукта) — только ссылки и подписка.

## Логи

- **Локально** (`python -m vpn_bot`): всё пишется в **тот же терминал** (stdout), уровень `INFO`.
- **Через systemd** (если сделаешь юнит): `journalctl -u <имя-сервиса> -f`.

Логи **`vpn-productd`**: см. `VPN_PRODUCT_LOG_FILE` в `deploy/env/vpn-productd.env.example` (файл или stdout).

## Запуск

```bash
cd bot
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
cp .env.example .env
# заполнить BOT_TOKEN, VPN_API_*, ADMIN_IDS
python -m vpn_bot
```

## Docker

Из корня репозитория:

```bash
docker build -f bot/Dockerfile -t vpn-bot ./bot
```

## Старый бот

Предыдущая реализация на `python-telegram-bot` лежит в **`archive/telegram-bot-legacy/`** (не запускается из корня). Актуальный код — этот каталог `bot/`.
