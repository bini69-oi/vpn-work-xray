# VPN Product README (Практический)

Этот файл описывает **твой рабочий контур**: `vpn-productd` + `3x-ui` + Happ.
Не про upstream Xray в целом, а про то, как у тебя реально запущено и используется.

## 1) Что это

- `vpn-productd` — API-слой продукта (подписки, профили, квоты, выдача ссылок).
- `3x-ui` — панель и Xray runtime (inbound на `8443`).
- `caddy` — HTTPS прокси для публичной подписки и панели.
- Happ — клиент, который забирает подписку по URL.

Текущая модель:
- На каждого пользователя выдается персональная подписка.
- Автоматом: `30 дней + 1 TB`.
- Лимиты применяются на пользователя, не на порт.

---

## 2) Базовые пути на сервере

- API env: `/etc/vpn-product/vpn-productd.env`
- Product DB: `/var/lib/vpn-product/product.db`
- 3x-ui DB: `/etc/x-ui/x-ui.db`
- Runtime config x-ui/xray: `/usr/local/x-ui/bin/config.json`
- Caddy config: `/etc/caddy/Caddyfile`

---

## 3) Сервисы

Проверка статуса:

```bash
systemctl is-active vpn-productd x-ui caddy
```

Рестарт:

```bash
systemctl restart vpn-productd x-ui caddy
```

---

## 4) Выдать новую подписку пользователю

### 4.1 Быстрый вариант (через API)

```bash
ADMIN_TOKEN=$(sed -n 's/^VPN_PRODUCT_ADMIN_TOKEN=//p' /etc/vpn-product/vpn-productd.env | tr -d '\r\n')
curl -sS -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"userId":"new_user_1","source":"manual"}' \
  http://127.0.0.1:8080/admin/issue/link
```

Ответ вернет:
- `url` — ссылку подписки
- `days: 30`
- `appliedTo3xui: true`
- `profileId: user-...`

### 4.2 Важно

- Повторная выдача для того же `userId` ревокает старую активную подписку и делает новую.
- UUID пользователя стабилизирован (для одного userId не должен постоянно меняться).

---

## 5) Проверить, что у пользователя реально 1TB/30 дней

### 5.1 В product

```bash
python3 - <<'PY'
import sqlite3
c=sqlite3.connect('/var/lib/vpn-product/product.db')
cur=c.cursor()
for r in cur.execute("SELECT user_id, expires_at, revoked FROM subscriptions ORDER BY created_at DESC LIMIT 10"):
    print(r)
c.close()
PY
```

### 5.2 В 3x-ui (`client_traffics`)

```bash
python3 - <<'PY'
import sqlite3
c=sqlite3.connect('/etc/x-ui/x-ui.db')
cur=c.cursor()
for r in cur.execute("SELECT email,total,expiry_time,enable,up,down FROM client_traffics ORDER BY id DESC LIMIT 20"):
    print(r)
c.close()
PY
```

`total = 1099511627776` означает 1 TB.

---

## 6) Почему иногда "не работает ссылка"

Частые причины:
- В Happ остался старый configID (локальный кэш).
- Пользователь есть в `x-ui.db`, но runtime еще не перечитал конфиг.

Что делать:
1. Удалить подписку из Happ.
2. Полностью закрыть Happ.
3. Добавить свежий URL подписки заново.
4. Перезапустить `x-ui` при необходимости:

```bash
systemctl restart x-ui
```

---

## 7) Автосинк, чтобы не править руками

### 7.1 Runtime sync (`db -> runtime`)

Таймер:
- `vpn-xui-runtime-sync.timer`

Проверка:

```bash
systemctl status vpn-xui-runtime-sync.timer --no-pager
```

### 7.2 Синк отображаемых лимитов в 3x-ui UI

Таймер:
- `vpn-xui-client-limits-sync.timer`

Проверка:

```bash
systemctl status vpn-xui-client-limits-sync.timer --no-pager
```

---

## 8) Массово выставить всем 1TB + 30 дней

Скрипт:
- `deploy/scripts/bulk_set_1tb_30days.py`

Запуск на сервере:

```bash
python3 /root/bulk_set_1tb_30days.py
systemctl restart x-ui
```

Проверка:

```bash
python3 /root/verify_user_limits.py
```

---

## 9) Чистка тестовых пользователей

Скрипт:
- `deploy/scripts/cleanup_users.py`

В текущем виде оставляет whitelist-пользователей. Использовать осторожно.

---

## 10) Временный smoke-пользователь (создать/проверить/удалить)

Скрипт:
- `deploy/scripts/test_ephemeral_user.sh`

Пример:

```bash
TEST_USER=test_once_user /root/test_ephemeral_user.sh
```

---

## 11) Бэкап перед удалением сервера

Локально у тебя уже сохранен полный архив миграции:

- `deploy/backups/vpn-migration-full-20260330-212536.tar.gz`
- `deploy/backups/vpn-migration-full-20260330-212536.tar.gz.sha256`

Это включает:
- БД продукта и панели
- env/config
- service/timer
- бинарники/скрипты, нужные для восстановления

---

## 12) Минимальный чек-лист перед удалением старого сервера

1. Проверен локальный SHA256 архива.
2. Архив продублирован во второе хранилище (облако/диск).
3. Понимание, что после удаления старого сервера без restore-процесса пользователи не восстановятся сами.

---

## 13) Быстрые команды-шпаргалки

Выдать ссылку:

```bash
ADMIN_TOKEN=$(sed -n 's/^VPN_PRODUCT_ADMIN_TOKEN=//p' /etc/vpn-product/vpn-productd.env | tr -d '\r\n')
curl -sS -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"userId":"new_user_x","source":"manual"}' \
  http://127.0.0.1:8080/admin/issue/link
```

Статус сервисов:

```bash
systemctl is-active vpn-productd x-ui caddy
```

Перезапуск всего контура:

```bash
systemctl restart vpn-productd x-ui caddy
```

---

## 14) Где смотреть API

- `API.md` — полный список endpoint-ов и форматов.

