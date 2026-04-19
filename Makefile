PYTHON ?= python3
BOT_DIR := apps/vpn-telegram-bot
BOT_PY  := $(BOT_DIR)/.venv/bin/python
BOT_PIP := $(BOT_DIR)/.venv/bin/pip

.PHONY: help bot-venv bot bot-test bot-cov bot-lint bot-typecheck verify secret-scan ci clean

help:
	@echo "VPN Product (Remnawave + Telegram bot) — Makefile targets"
	@echo
	@echo "  make bot-venv       — создать apps/vpn-telegram-bot/.venv с runtime + dev-зависимостями"
	@echo "  make bot            — запустить Telegram-бот (Remnawave backend)"
	@echo "  make bot-test       — pytest (быстро, без покрытия)"
	@echo "  make bot-cov        — pytest + покрытие (порог 80%)"
	@echo "  make bot-lint       — ruff линт (apps/vpn-telegram-bot)"
	@echo "  make bot-typecheck  — mypy на vpn_bot/"
	@echo "  make verify         — secret-scan + lint + typecheck + cov (то, что гоняет CI)"
	@echo "  make secret-scan    — поиск утечек секретов в коде"

bot-venv:
	cd $(BOT_DIR) && $(PYTHON) -m venv .venv \
		&& ./.venv/bin/pip install -U pip \
		&& ./.venv/bin/pip install -r requirements.txt \
		&& ./.venv/bin/pip install -r requirements-dev.txt

bot:
	@test -f $(BOT_PY) || (echo "Сначала выполни: make bot-venv" && exit 1)
	cd $(BOT_DIR) && ./.venv/bin/python -m vpn_bot

bot-test:
	@test -f $(BOT_PY) || (echo "Сначала выполни: make bot-venv" && exit 1)
	cd $(BOT_DIR) && ./.venv/bin/python -m pytest -q

bot-cov:
	@test -f $(BOT_PY) || (echo "Сначала выполни: make bot-venv" && exit 1)
	cd $(BOT_DIR) && ./.venv/bin/python -m pytest --cov=vpn_bot --cov-report=term-missing --cov-fail-under=80

bot-lint:
	@test -f $(BOT_PY) || (echo "Сначала выполни: make bot-venv" && exit 1)
	cd $(BOT_DIR) && ./.venv/bin/python -m ruff check vpn_bot tests

bot-typecheck:
	@test -f $(BOT_PY) || (echo "Сначала выполни: make bot-venv" && exit 1)
	cd $(BOT_DIR) && ./.venv/bin/python -m mypy vpn_bot

secret-scan:
	$(PYTHON) scripts/secret_scan.py

verify: secret-scan bot-lint bot-typecheck bot-cov

ci: verify

clean:
	rm -rf $(BOT_DIR)/.venv $(BOT_DIR)/.pytest_cache $(BOT_DIR)/.mypy_cache $(BOT_DIR)/.ruff_cache $(BOT_DIR)/.coverage
