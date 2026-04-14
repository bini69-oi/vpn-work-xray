# Архив

- **`telegram-bot-legacy/`** — прежний бот на `python-telegram-bot` (перенесён из корня `telegram-bot/`). Не используется в активном потоке; актуальный бот — **`apps/vpn-telegram-bot/`** (aiogram 3).
- **`tests-integration-coverage/`** — старые bash-скрипты полного покрытия из апстримного дерева Xray (`coverall`, `coverall2`). В этом репозитории не вызываются из `Makefile` и CI; актуальное покрытие — цели `make cover` / `make verify`.
- **`config-examples/`** — пример профиля (`secure_profile.json`), ранее лежал в `internal/examples/` и ни на что в коде не ссылался.

Восстановление в корень (если понадобится):

```bash
mv archive/telegram-bot-legacy telegram-bot
```
