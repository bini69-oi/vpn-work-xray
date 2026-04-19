# Приложения (`apps/`)

Клиентские сервисы вокруг Remnawave Panel.

| Каталог | Назначение |
|---------|------------|
| [`vpn-telegram-bot/`](vpn-telegram-bot/README.md) | Telegram-бот на Python (aiogram 3, пакет `vpn_bot`); ходит в Remnawave Panel REST API |

Запуск и переменные окружения — в README подпроекта. Из корня репозитория удобно использовать `make bot` / `make bot-venv` / `make bot-test`.

Mini App (legacy) переехал в [`archive/telegram-miniapp/`](../archive/telegram-miniapp/README.md) — он работал только со старым `vpn-productd` и не переписан на Remnawave.
