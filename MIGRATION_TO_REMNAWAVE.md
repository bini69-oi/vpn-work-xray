## Миграция на Remnawave

Мы переходим с текущей архитектуры на базе **`vpn-productd` + 3x-ui** к **Remnawave Panel + Remnawave Node**.

### Почему

- Remnawave разделяет управление (Panel) и трафик (Node), и поддерживает актуальный workflow управления нодами/подписками.
- `vpn-productd` и связанная доменная логика продукта выводятся из эксплуатации.

### Текущий статус

**Этап 1 из N (подготовка репозитория к деплою Remnawave):**

- добавлены docker-compose/env/скрипты для Remnawave Panel + Postgres + subscription-page
- добавлены docker-compose/env/скрипты для Remnawave Node (отдельный сервер)
- добавлен systemd backup timer для бэкапов Postgres базы Remnawave (retention 14 дней)
- старый код не удаляется, но помечается к удалению на следующем этапе

### Чего ещё нет (будет в следующих этапах)

- Telegram-бот
- Оплата/платёжная интеграция
- Миграция старых пользователей/данных

### Где смотреть инструкции деплоя

- `deploy/remnawave/README.md`

