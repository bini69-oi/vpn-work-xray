# VPN Telegram Mini App

Telegram Mini App для управления VPN-подписками. Работает с `vpn-productd` API.

## Что умеет

- 🚀 Получить VPN-подписку в один тап
- 📊 Статус: дни, трафик, дата истечения
- 🔄 Обновить/продлить подписку
- 📋 Скопировать ссылку подписки для Happ
- 📖 Инструкция по подключению

## Структура

```
apps/telegram-miniapp/
├── index.js              # Точка входа (dotenv + старт сервера)
├── src/
│   ├── server.js         # Express, статика, маршруты
│   ├── config.js         # Переменные окружения
│   ├── telegram-init-data.js
│   ├── vpn-product-client.js
│   ├── optional-polling-bot.js
│   ├── middleware/
│   │   └── telegram-user.js
│   └── routes/
│       └── api.js
├── webapp/
│   └── index.html        # Mini App фронтенд (single-file)
├── .env.example          # Шаблон переменных окружения
├── package.json
└── README.md
```

## Быстрый старт

### 1. Создай Telegram бота

1. Открой [@BotFather](https://t.me/BotFather) в Telegram
2. Отправь `/newbot`
3. Введи имя бота (например: `My VPN Bot`)
4. Введи username (например: `my_vpn_bot`)
5. Скопируй **токен** (формат: `123456:ABC-DEF...`)

### 2. Настрой Mini App в BotFather

1. Отправь `/mybots` → выбери бота → **Bot Settings** → **Menu Button**
2. Установи URL меню (твой домен с HTTPS)
3. Или: `/mybots` → бот → **Bot Settings** → **Mini App** → задай URL

### 3. Получи HTTPS (обязательно для Mini App)

Telegram требует HTTPS. Варианты для разработки:

**Вариант A — ngrok (быстро для теста):**
```bash
# Установи ngrok: https://ngrok.com
ngrok http 3000
# Скопируй HTTPS URL (например: https://abc123.ngrok.io)
```

**Вариант B — Cloudflare Tunnel:**
```bash
cloudflared tunnel --url http://localhost:3000
```

**Вариант C — свой сервер с Caddy/nginx + Let's Encrypt**

### 4. Настрой переменные окружения

```bash
cp .env.example .env
```

Заполни `.env`:
```env
BOT_TOKEN=123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11
VPN_API_URL=http://localhost:8080
VPN_ADMIN_TOKEN=твой_admin_токен_из_vpn-productd
WEBAPP_URL=https://abc123.ngrok.io
PORT=3000
```

`VPN_ADMIN_TOKEN` — берётся из `/etc/vpn-product/vpn-productd.env` на сервере.

### 5. Запусти

```bash
npm install
npm start
```

По умолчанию **только** Express (статика + `/api/*`). Основной бот — Python в **`apps/vpn-telegram-bot/`** (aiogram). URL миниаппа в Telegram задай в **@BotFather** (Menu Button / Mini App) на тот же **HTTPS**, что `WEBAPP_URL`. Чтобы снова поднять polling в Node, задай `START_TELEGRAM_BOT_POLLING=1`.

### 6. Проверь

1. Открой бота в Telegram (Python-бот)
2. Нажми «📱 Приложение» (или отправь `/start` и открой WebApp из меню)
3. Mini App должен загрузиться по HTTPS

## Деплой на сервер (рядом с vpn-productd)

```bash
# В каталоге репозитория
cd apps/telegram-miniapp
npm install --production

# Секреты — в /etc/vpn-product/vpn-tg-miniapp.env (см. .env.example)
sudo systemctl enable --now vpn-tg-miniapp.service
```

Юнит в репозитории: `deploy/systemd/vpn-tg-miniapp.service` (скопируй в `/etc/systemd/system/` и поправь `WorkingDirectory` под путь к `apps/telegram-miniapp` на сервере).

### Добавь в Caddy (для HTTPS)

В `/etc/caddy/Caddyfile` добавь блок:

```
tg.your-domain.com {
    reverse_proxy localhost:3000
}
```

```bash
sudo systemctl reload caddy
```

Затем установи `WEBAPP_URL=https://tg.your-domain.com` в `.env` и перезапусти.

## API Endpoints (внутренние)

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/status` | Статус подписки пользователя |
| POST | `/api/subscribe` | Создать/обновить подписку |
| GET | `/api/profile` | Профиль пользователя |
| GET | `/api/health` | Health check |

Все endpoints (кроме health) требуют заголовок `X-Telegram-Init-Data` — передаётся автоматически из Mini App SDK.

## Безопасность

- Авторизация через [Telegram initData](https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app) с HMAC-SHA256 валидацией
- API-сервер проксирует запросы к `vpn-productd` с admin-токеном
- Токен бота и admin-токен никогда не попадают в фронтенд
- Mini App userId: `tg_{telegram_user_id}`

## Отладка

Добавь `?debug` к URL для открытия без Telegram:
```
http://localhost:3000/?debug
```

В debug-режиме авторизация не пройдёт (401), но можно проверить UI.
