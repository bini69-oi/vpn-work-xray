# VPN Product — руководство по проекту

Этот репозиторий содержит VPN-продукт поверх `xray-core`: API-сервис, управление профилями, синхронизацию с `3x-ui`, диагностику и скрипты деплоя.

Документ ниже объясняет, что входит в систему, как она работает и как ее запускать/обслуживать.

## 1. Что входит в продукт

- `vpn-productd` — API-слой продукта (подписки, профили, лимиты, служебные операции).
- `xray-core` — сетевой движок и runtime.
- `3x-ui` — панель и хранение клиентских записей.
- `caddy` — внешний HTTPS reverse proxy.
- `sqlite` — локальное хранилище состояния продукта (`product.db`).

## 2. Архитектура на высоком уровне

1. Клиент обращается к API `vpn-productd`.
2. API работает с профилями/подписками в `product.db`.
3. При необходимости профиль синхронизируется в `3x-ui`.
4. Генерируется runtime-конфиг для Xray.
5. Xray применяет конфигурацию и обслуживает трафик.

Сопутствующие таймеры (systemd) поддерживают регулярный sync и служебные задачи.

## 3. Важные пути на сервере

- `/etc/vpn-product/vpn-productd.env` — переменные окружения API.
- `/var/lib/vpn-product/product.db` — база продукта.
- `/etc/x-ui/x-ui.db` — база `3x-ui`.
- `/usr/local/x-ui/bin/config.json` — runtime-конфиг `x-ui/xray`.
- `/etc/caddy/Caddyfile` — публичный reverse proxy.

## 4. Основные сервисы

Проверка:

```bash
systemctl is-active vpn-productd x-ui caddy
```

Перезапуск:

```bash
systemctl restart vpn-productd x-ui caddy
```

## 5. Быстрый старт для разработки

Требования:

- Go (актуальная версия из `go.mod`)
- `golangci-lint` (или запуск через `make lint`)

Проверки:

```bash
make test
make lint
make cover
make verify
```

## 6. CI и качество

В проекте используется workflow CI и цели `Makefile`.

Рекомендуемый локальный минимум перед push:

```bash
make verify
```

Если нужен более полный прогон:

```bash
MIN_COVERAGE=60 make verify
```

Чтобы проверки запускались автоматически на каждый commit/push:

```bash
bash scripts/install_git_hooks.sh
```

## 7. Работа с подписками и пользователями

Типовой сценарий:

1. Создать/обновить подписку через API.
2. Проверить запись в `product.db`.
3. Проверить, что пользователь присутствует в `x-ui`.
4. Убедиться, что runtime-конфиг актуален.

Полезные скрипты в `deploy/scripts/`:

- `sync_xui_to_product.sh`
- `sync_xui_usage_to_product.sh`
- `verify_user_limits.py`
- `cleanup_users.py`
- `test_ephemeral_user.sh`

## 8. Миграции и восстановление

Для экспорта/импорта состояния используются:

- `deploy/scripts/export_server_state.sh`
- `deploy/scripts/import_server_state.sh`
- `deploy/scripts/migrate_from_recovered_state.sh`

Во время миграции обязательно указывать целевой IP явно:

```bash
bash deploy/scripts/migrate_from_recovered_state.sh <new_ip>
```

## 9. Ежедневные резервные копии

В ветке продукта добавлены systemd-юниты для ежедневного backup в `00:00`.

Установка:

```bash
sudo bash deploy/scripts/install_backup_timer.sh
```

Проверка:

```bash
systemctl status vpn-product-backup.timer --no-pager
systemctl status vpn-product-backup.service --no-pager
ls -lah /var/backups/vpn-product
```

По умолчанию:

- архив + `.sha256` создаются автоматически;
- старые копии удаляются по retention (14 дней).

## 10. Диагностика и отладка

Для точечной проверки пользователя:

```bash
python3 deploy/scripts/diag_user_sync.py <user_id>
```

Если `user_id` не передан, используется безопасное тестовое значение.

## 11. Практические рекомендации

- Не коммитить персональные данные и реальные production-IP.
- Не хранить секреты в репозитории.
- Перед деплоем всегда делать backup.
- После деплоя запускать smoke-проверки.

## DR drill и SLO

- DR rehearsal и шаблон отчета: `DR_DRILL_RUNBOOK.md`
- SLO/SLI и baseline нагрузочных прогонов: `SLO_BASELINE.md`
- Базовый baseline: `benchmarks/baseline/current.json`

## 12. Структура репозитория (кратко)

- `product/` — доменная логика продукта и API.
- `deploy/` — systemd-юниты, env-файлы, операционные скрипты.
- `app/`, `core/`, `proxy/`, `transport/`, `features/` — компоненты `xray-core`.
- `testing/` — интеграционные и сценарные тесты.

## 13. Лицензия и источник ядра

Проект основан на `xray-core` и сохраняет совместимость с исходной архитектурой ядра.
Подробности по лицензии смотри в файле `LICENSE`.
