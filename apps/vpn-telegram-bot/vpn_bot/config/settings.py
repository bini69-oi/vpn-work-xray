from __future__ import annotations

from pathlib import Path

from pydantic import AliasChoices, Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

# Каталог приложения (родитель пакета `vpn_bot`), например `apps/vpn-telegram-bot/`.
_BOT_ROOT = Path(__file__).resolve().parents[2]


def _parse_id_list(raw: str) -> list[int]:
    out: list[int] = []
    for raw_part in (raw or "").split(","):
        part = raw_part.strip()
        if not part:
            continue
        if part.isdigit() or (part.startswith("-") and part[1:].isdigit()):
            try:
                out.append(int(part))
            except ValueError:
                continue
    return out


class Settings(BaseSettings):
    """Конфигурация Telegram-бота (Remnawave-only)."""

    model_config = SettingsConfigDict(
        env_file=_BOT_ROOT / ".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # --- Telegram ---
    bot_token: str = Field(
        validation_alias=AliasChoices("BOT_TOKEN", "TELEGRAM_BOT_TOKEN"),
    )
    bot_username: str = Field(default="", validation_alias="BOT_USERNAME")

    admin_ids_csv: str = Field(
        default="",
        validation_alias=AliasChoices("ADMIN_IDS", "ADMIN_TELEGRAM_IDS"),
    )
    allowed_ids_csv: str = Field(default="", validation_alias="ALLOWED_TELEGRAM_IDS")

    # HTTP(S)/SOCKS5 proxy для api.telegram.org (если сеть режет Telegram).
    telegram_proxy_url: str = Field(
        default="",
        validation_alias=AliasChoices("TELEGRAM_PROXY", "TELEGRAM_HTTPS_PROXY"),
    )

    # --- Remnawave Panel API ---
    # Публичный URL панели без `/api` — клиент сам допишет путь.
    remnawave_panel_url: str = Field(
        default="",
        validation_alias=AliasChoices("REMNAWAVE_PANEL_URL", "REMNAWAVE_BASE_URL"),
    )
    remnawave_api_token: str = Field(default="", validation_alias="REMNAWAVE_API_TOKEN")
    # Если панель за Caddy с X-Api-Key.
    remnawave_caddy_token: str = Field(default="", validation_alias="REMNAWAVE_CADDY_TOKEN")
    # UUID внутренних squad через запятую (обязательно для создания пользователя).
    remnawave_internal_squads_raw: str = Field(
        default="",
        validation_alias=AliasChoices(
            "REMNAWAVE_INTERNAL_SQUAD_UUIDS",
            "REMNAWAVE_INTERNAL_SQUAD_UUID",
        ),
    )

    # --- Локальная SQLite бота (счётчики, рефералы, заявки на оплату) ---
    database_path: str = Field(default="data/bot.db", validation_alias="DATABASE_PATH")

    # --- Реферальная программа ---
    referral_bonus_days: int = Field(default=15, validation_alias="REFERRAL_BONUS_DAYS")

    # --- Платежи ---
    payment_manual_details: str = Field(default="", validation_alias="PAYMENT_MANUAL_DETAILS")
    payment_provider_token: str = Field(default="", validation_alias="PAYMENT_PROVIDER_TOKEN")
    payment_currency: str = Field(default="RUB", validation_alias="PAYMENT_CURRENCY")

    # Страница оплаты СБП (HTTPS). Пусто — кнопка-заглушка с подсказкой.
    sbp_pay_url: str = Field(default="", validation_alias=AliasChoices("SBP_PAY_URL", "SBP_PAYMENT_URL"))

    # Заглушка проверки оплаты: при PAYMENT_STUB=1 не создаётся заявка админу.
    payment_stub_enabled: bool = Field(default=False, validation_alias="PAYMENT_STUB")
    payment_stub_result: str = Field(default="ok", validation_alias="PAYMENT_STUB_RESULT")

    crypto_wallet_address: str = Field(default="", validation_alias="CRYPTO_WALLET_ADDRESS")

    # --- Поддержка ---
    support_username: str = Field(default="", validation_alias="SUPPORT_USERNAME")

    @field_validator("payment_stub_enabled", mode="before")
    @classmethod
    def _payment_stub_bool(cls, v: object) -> bool:
        if v is True:
            return True
        if v is False or v is None:
            return False
        s = str(v).strip().lower()
        if s in ("0", "false", "no", "off", ""):
            return False
        return s in ("1", "true", "yes", "on")

    @field_validator("payment_stub_result", mode="before")
    @classmethod
    def _payment_stub_result(cls, v: object) -> str:
        s = str(v or "ok").strip().lower()
        return s if s in ("ok", "fail") else "ok"

    @field_validator("referral_bonus_days", mode="before")
    @classmethod
    def _bonus_days(cls, v: object) -> int:
        try:
            n = int(v)
        except (TypeError, ValueError):
            return 15
        return max(1, min(n, 3650))

    def admin_ids(self) -> set[int]:
        return set(_parse_id_list(self.admin_ids_csv))

    def allowed_ids(self) -> set[int] | None:
        ids = _parse_id_list(self.allowed_ids_csv)
        return set(ids) if ids else None

    def remnawave_internal_squad_uuids(self) -> list[str]:
        return [p.strip() for p in (self.remnawave_internal_squads_raw or "").split(",") if p.strip()]

    def api_configured(self) -> bool:
        return bool(self.remnawave_api_token.strip() and self.remnawave_panel_url.strip())

    def database_file(self) -> Path:
        p = Path(self.database_path)
        if not p.is_absolute():
            p = _BOT_ROOT / p
        return p


settings = Settings()  # type: ignore[call-arg]
