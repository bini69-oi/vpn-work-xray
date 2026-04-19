# Commands cheat-sheet

Все рабочие команды для Remnawave-стека.

## Telegram-бот

Из корня репозитория:

```bash
make bot-venv       # один раз: .venv + runtime + dev-зависимости
make bot            # запустить бот
make bot-test       # pytest (быстро)
make bot-cov        # pytest + покрытие (порог 80%)
make bot-lint       # ruff
make bot-typecheck  # mypy
make verify         # secret-scan + lint + typecheck + cov (то, что гоняет CI)
```

Локальный запуск без Makefile:

```bash
cd apps/vpn-telegram-bot
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt -r requirements-dev.txt
cp .env.example .env
# заполни BOT_TOKEN, ADMIN_IDS, REMNAWAVE_PANEL_URL, REMNAWAVE_API_TOKEN, REMNAWAVE_INTERNAL_SQUAD_UUIDS
python -m vpn_bot
```

## Remnawave Panel + Node (production)

См. полный гайд: [`deploy/remnawave/README.md`](../deploy/remnawave/README.md).

Сервер панели:

```bash
sudo bash deploy/remnawave/scripts/install_panel.sh
sudoedit /opt/remnawave/.env   # PANEL_DOMAIN, JWT_*, SUB_PUBLIC_DOMAIN, POSTGRES_PASSWORD
cd /opt/remnawave && docker compose pull && docker compose up -d
```

Сервер ноды:

```bash
sudo bash deploy/remnawave/scripts/install_node.sh
sudoedit /opt/remnanode/.env   # NODE_PORT, SECRET_KEY (взять в UI панели → Add Node)
cd /opt/remnanode && docker compose pull && docker compose up -d
```

Бэкап Postgres:

```bash
sudo bash deploy/remnawave/scripts/backup_panel.sh   # см. README про cron/timer
```

## Полезные API-вызовы Remnawave

```bash
PANEL=https://panel.example.com
TOKEN=$(cat /etc/remnawave-bot/token)

curl -sS -H "Authorization: Bearer $TOKEN" "$PANEL/api/system/health" | jq
curl -sS -H "Authorization: Bearer $TOKEN" "$PANEL/api/users/by-telegram-id/123456789" | jq
```

## Telegram bot — debug

```bash
# из подкаталога apps/vpn-telegram-bot/
.venv/bin/python -m vpn_bot   # запуск с тем же терминалом для логов

# конкретный pytest
.venv/bin/python -m pytest tests/test_remnawave_client.py -v
```

## Старый стек

Команды для `vpn-productd` / 3x-ui — в [`archive/vpn-productd/docs/`](../archive/vpn-productd/docs/).
