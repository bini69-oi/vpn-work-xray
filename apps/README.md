# Приложения (`apps/`)

Здесь лежат **клиентские сервисы** вокруг продукта (не ядро Xray и не `vpn-productd` — они остаются в корне и в `internal/`).

| Каталог | Назначение |
|---------|------------|
| [`vpn-telegram-bot/`](vpn-telegram-bot/README.md) | Telegram-бот на Python (aiogram 3), пакет `vpn_bot` |
| [`telegram-miniapp/`](telegram-miniapp/README.md) | Mini App: Node (Express), прокси к API и статика `webapp/` |

Запуск и переменные окружения описаны в README каждого подпроекта. Из корня репозитория для бота удобно использовать `make bot` / `make bot-venv`.
