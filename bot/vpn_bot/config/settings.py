from __future__ import annotations

from pathlib import Path

from pydantic import AliasChoices, Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

# Каталог `bot/` (на уровень выше пакета `vpn_bot`).
_BOT_ROOT = Path(__file__).resolve().parents[2]


def _parse_id_list(raw: str) -> list[int]:
    out: list[int] = []
    for part in (raw or "").split(","):
        part = part.strip()
        if not part:
            continue
        if part.isdigit() or (part.startswith("-") and part[1:].isdigit()):
            try:
                out.append(int(part))
            except ValueError:
                continue
    return out


def _parse_profile_ids(raw: str) -> list[str] | None:
    raw = (raw or "").strip()
    if not raw:
        return None
    ids = [p.strip() for p in raw.split(",") if p.strip()]
    return ids if ids else None


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=_BOT_ROOT / ".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # Совместимость с legacy-ботом: TELEGRAM_BOT_TOKEN, VPN_PRODUCT_BASE_URL, VPN_PRODUCT_API_TOKEN, …
    bot_token: str = Field(
        validation_alias=AliasChoices("BOT_TOKEN", "TELEGRAM_BOT_TOKEN"),
    )

    admin_ids_csv: str = Field(
        default="",
        validation_alias=AliasChoices("ADMIN_IDS", "ADMIN_TELEGRAM_IDS"),
    )
    allowed_ids_csv: str = Field(default="", validation_alias="ALLOWED_TELEGRAM_IDS")

    vpn_api_url: str = Field(
        default="http://127.0.0.1:8080",
        validation_alias=AliasChoices("VPN_API_URL", "VPN_PRODUCT_BASE_URL"),
    )
    vpn_api_token: str = Field(
        default="",
        validation_alias=AliasChoices("VPN_API_TOKEN", "VPN_PRODUCT_API_TOKEN"),
    )
    vpn_profile_ids_raw: str = Field(default="", validation_alias="VPN_PROFILE_IDS")

    database_path: str = Field(default="data/bot.db", validation_alias="DATABASE_PATH")

    referral_bonus_days: int = Field(default=15, validation_alias="REFERRAL_BONUS_DAYS")

    payment_manual_details: str = Field(default="", validation_alias="PAYMENT_MANUAL_DETAILS")
    payment_provider_token: str = Field(default="", validation_alias="PAYMENT_PROVIDER_TOKEN")
    payment_currency: str = Field(default="RUB", validation_alias="PAYMENT_CURRENCY")

    # Страница оплаты СБП (HTTPS). Пусто — кнопка-заглушка с подсказкой.
    sbp_pay_url: str = Field(default="", validation_alias=AliasChoices("SBP_PAY_URL", "SBP_PAYMENT_URL"))

    # Заглушка проверки оплаты: при PAYMENT_STUB=1 не создаётся заявка админу.
    payment_stub_enabled: bool = Field(default=False, validation_alias="PAYMENT_STUB")
    payment_stub_result: str = Field(default="ok", validation_alias="PAYMENT_STUB_RESULT")

    crypto_wallet_address: str = Field(default="", validation_alias="CRYPTO_WALLET_ADDRESS")

    support_username: str = Field(default="", validation_alias="SUPPORT_USERNAME")
    bot_username: str = Field(default="", validation_alias="BOT_USERNAME")

    # HTTP(S)-прокси до api.telegram.org (если сеть режет Telegram). Пример: http://127.0.0.1:7890
    telegram_proxy_url: str = Field(
        default="",
        validation_alias=AliasChoices("TELEGRAM_PROXY", "TELEGRAM_HTTPS_PROXY"),
    )

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
            n = int(v)  # type: ignore[arg-type]
        except (TypeError, ValueError):
            return 15
        return max(1, min(n, 3650))

    def admin_ids(self) -> set[int]:
        return set(_parse_id_list(self.admin_ids_csv))

    def allowed_ids(self) -> set[int] | None:
        ids = _parse_id_list(self.allowed_ids_csv)
        return set(ids) if ids else None

    def profile_ids(self) -> list[str] | None:
        return _parse_profile_ids(self.vpn_profile_ids_raw)

    def database_file(self) -> Path:
        p = Path(self.database_path)
        if not p.is_absolute():
            p = _BOT_ROOT / p
        return p


settings = Settings()
