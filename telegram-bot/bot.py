"""
Telegram-бот выдачи подписок через VPN Product API (POST /v1/issue/link и связанные GET).
"""

from __future__ import annotations

import json
import logging
import os
import re
from typing import Any
from urllib.parse import urlparse

import httpx
from dotenv import load_dotenv
from telegram import InlineKeyboardButton, InlineKeyboardMarkup, Update
from telegram.ext import Application, CallbackQueryHandler, CommandHandler, ContextTypes

load_dotenv()

logging.basicConfig(
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    level=logging.INFO,
)
logger = logging.getLogger(__name__)
# Не логировать полный URL с токеном бота (httpx INFO пишет request line).
logging.getLogger("httpx").setLevel(logging.WARNING)
logging.getLogger("httpcore").setLevel(logging.WARNING)

HTTP_TIMEOUT = httpx.Timeout(60.0)


def _setup_loki_logging() -> None:
    """Опциональная отправка логов в Loki (если настроен LOKI_URL)."""
    loki_url = os.environ.get("LOKI_URL", "").strip()
    if not loki_url:
        return

    try:
        from logging_loki import LokiHandler
        class QuietLokiHandler(LokiHandler):
            def handleError(self, record: logging.LogRecord) -> None:  # noqa: N802
                # Loki может быть временно недоступен; не заспамливаем stderr traceback'ами.
                return

        tags_raw = os.environ.get("LOKI_TAGS", "").strip()
        tags: dict[str, str] = {}
        if tags_raw:
            for part in tags_raw.split(","):
                part = part.strip()
                if not part or "=" not in part:
                    continue
                k, v = part.split("=", 1)
                k = k.strip()
                v = v.strip()
                if k and v:
                    tags[k] = v
        tags.setdefault("service", "telegram-bot")

        auth = None
        username = os.environ.get("LOKI_USERNAME", "").strip()
        password = os.environ.get("LOKI_PASSWORD", "").strip()
        if username or password:
            auth = (username, password)

        loki_handler = QuietLokiHandler(
            url=loki_url,
            tags=tags,
            auth=auth,
            version="1",
        )
        loki_handler.setLevel(logging.INFO)
        loki_handler.setFormatter(
            logging.Formatter("%(asctime)s %(levelname)s %(name)s %(message)s"),
        )
        logging.getLogger().addHandler(loki_handler)
        logger.info("Loki logging enabled")
    except Exception as exc:
        logger.warning("Loki logging disabled: %s", exc)


def _env_truthy(name: str) -> bool:
    v = os.environ.get(name, "").strip().lower()
    return v in ("1", "true", "yes", "on")


def _dry_run_enabled() -> bool:
    return _env_truthy("BOT_DRY_RUN") or _env_truthy("TELEGRAM_BOT_DRY_RUN")


def _vpn_user_id(telegram_user_id: int) -> str:
    return f"tg_{telegram_user_id}"


def _parse_allowed_ids(raw: str | None) -> set[int] | None:
    if raw is None or not str(raw).strip():
        return None
    out: set[int] = set()
    for part in str(raw).split(","):
        part = part.strip()
        if not part:
            continue
        try:
            out.add(int(part))
        except ValueError:
            logger.warning("ignored invalid ALLOWED_TELEGRAM_IDS entry: %r", part)
    return out if out else None


def _parse_profile_ids(raw: str | None) -> list[str] | None:
    if raw is None or not str(raw).strip():
        return None
    ids = [p.strip() for p in str(raw).split(",") if p.strip()]
    return ids if ids else None


def _display_name(update: Update) -> str:
    u = update.effective_user
    if not u:
        return "telegram-user"
    if u.username:
        return f"@{u.username}"
    parts = [p for p in (u.first_name, u.last_name) if p]
    return " ".join(parts) if parts else f"user-{u.id}"


def is_allowed(update: Update, allowed: set[int] | None) -> bool:
    if allowed is None:
        return True
    uid = update.effective_user.id if update.effective_user else None
    return uid is not None and uid in allowed


def mask_url_for_logs(url: str) -> str:
    """Не логировать полный токен в пути /public/subscriptions/<token>."""
    if "/public/subscriptions/" not in url:
        return url
    try:
        p = urlparse(url)
        path = p.path
        idx = path.rfind("/public/subscriptions/")
        if idx < 0:
            return url
        base = path[: idx + len("/public/subscriptions/")]
        tail = path[idx + len("/public/subscriptions/") :]
        if len(tail) > 6:
            tail = tail[:4] + "…"
        else:
            tail = "…"
        masked_path = base + tail
        return p._replace(path=masked_path).geturl()
    except Exception:
        return re.sub(r"(/public/subscriptions/)([^/?#]+)", r"\1…", url)


class VPNProductClient:
    """HTTP-клиент к vpn-productd."""

    def __init__(self, base_url: str, api_token: str) -> None:
        self._base = base_url.rstrip("/")
        self._headers = {"Authorization": f"Bearer {api_token}"}

    async def issue_link(
        self,
        user_id: str,
        name: str,
        source: str,
        profile_ids: list[str] | None,
        idempotency_key: str | None,
    ) -> tuple[int, dict[str, Any]]:
        body: dict[str, Any] = {
            "userId": user_id,
            "name": name,
            "source": source,
        }
        if profile_ids:
            body["profileIds"] = profile_ids
        headers = dict(self._headers)
        if idempotency_key:
            headers["X-Idempotency-Key"] = idempotency_key
        async with httpx.AsyncClient(timeout=HTTP_TIMEOUT) as client:
            r = await client.post(
                f"{self._base}/v1/issue/link",
                json=body,
                headers=headers,
            )
        try:
            data = r.json()
        except json.JSONDecodeError:
            data = {"error": r.text or f"HTTP {r.status_code}"}
        if not isinstance(data, dict):
            data = {"error": str(data)}
        return r.status_code, data

    async def issue_status(self, user_id: str) -> tuple[int, dict[str, Any]]:
        async with httpx.AsyncClient(timeout=HTTP_TIMEOUT) as client:
            r = await client.get(
                f"{self._base}/v1/issue/status",
                params={"userId": user_id},
                headers=dict(self._headers),
            )
        try:
            data = r.json()
        except json.JSONDecodeError:
            data = {"error": r.text or f"HTTP {r.status_code}"}
        if not isinstance(data, dict):
            data = {"error": str(data)}
        return r.status_code, data

    async def issue_history(self, user_id: str, limit: int = 10) -> tuple[int, dict[str, Any]]:
        async with httpx.AsyncClient(timeout=HTTP_TIMEOUT) as client:
            r = await client.get(
                f"{self._base}/v1/issue/history",
                params={"userId": user_id, "limit": str(limit)},
                headers=dict(self._headers),
            )
        try:
            data = r.json()
        except json.JSONDecodeError:
            data = {"error": r.text or f"HTTP {r.status_code}"}
        if not isinstance(data, dict):
            data = {"error": str(data)}
        return r.status_code, data


def main_keyboard() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        [
            [
                InlineKeyboardButton("Получить подписку", callback_data="issue"),
                InlineKeyboardButton("Статус", callback_data="status"),
            ],
            [InlineKeyboardButton("История", callback_data="history")],
        ]
    )


async def cmd_start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    allowed: set[int] | None = context.bot_data.get("allowed_ids")
    if not is_allowed(update, allowed):
        await update.message.reply_text("Доступ ограничен.")
        return
    extra = ""
    if context.bot_data.get("dry_run"):
        extra = (
            "\n\nРежим проверки без сервера (BOT_DRY_RUN=1): "
            "запросы к vpn-productd не выполняются."
        )
    text = (
        "Бот выдаёт персональную ссылку на подписку (через VPN Product API).\n\n"
        "Команды: /subscribe — выдать ссылку, /status — статус, /history — история выдач."
        + extra
    )
    await update.message.reply_text(text, reply_markup=main_keyboard())


async def cmd_help(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    await cmd_start(update, context)


def _api_error_message(data: dict[str, Any]) -> str:
    err = data.get("error") or data.get("message") or "неизвестная ошибка"
    code = data.get("code")
    if code:
        return f"{err} ({code})"
    return str(err)


async def _do_issue(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if context.bot_data.get("dry_run"):
        vpn_uid = _vpn_user_id(update.effective_user.id)
        name = _display_name(update)
        await _reply(
            update,
            "Режим проверки без сервера (BOT_DRY_RUN=1).\n\n"
            f"Ваш userId для API: {vpn_uid}\n"
            f"Имя для поля name: {name}\n\n"
            "Для реальной выдачи: задайте VPN_PRODUCT_BASE_URL и VPN_PRODUCT_API_TOKEN "
            "и уберите BOT_DRY_RUN из окружения.",
        )
        return

    client: VPNProductClient = context.bot_data["vpn_client"]
    profile_ids: list[str] | None = context.bot_data.get("profile_ids")
    uid = update.effective_user.id
    vpn_uid = _vpn_user_id(uid)
    name = _display_name(update)
    idem = str(update.update_id)
    try:
        status, data = await client.issue_link(
            vpn_uid,
            name=name,
            source="telegram",
            profile_ids=profile_ids,
            idempotency_key=idem,
        )
    except httpx.RequestError as e:
        await _reply(update, f"Сеть или сервер недоступны: {e}")
        return

    if status != 200:
        await _reply(update, f"Ошибка API ({status}): {_api_error_message(data)}")
        return

    url = (data.get("url") or "").strip()
    days = data.get("days", 30)
    applied = data.get("appliedTo3XUI")
    profile_id = data.get("profileId") or ""
    apply_err = (data.get("applyError") or "").strip()
    sub = data.get("subscription") or {}
    sub_id = sub.get("id", "")

    lines = [
        "Подписка выдана.",
        f"Срок: {days} дн.",
    ]
    if url:
        lines.append(f"Ссылка:\n{url}")
        logger.info(
            "issue ok user=%s subscriptionId=%s url=%s",
            vpn_uid,
            sub_id,
            mask_url_for_logs(url),
        )
    else:
        lines.append("Ссылка не пришла в ответе (проверьте public base URL на сервере).")
    if profile_id:
        lines.append(f"Профиль 3x-ui: {profile_id}")
    if applied is True:
        lines.append("Применено к 3x-ui: да")
    elif applied is False:
        lines.append("Применено к 3x-ui: нет")
    if apply_err:
        lines.append(f"Замечание: {apply_err}")

    await _reply(update, "\n".join(lines))


async def _do_status(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    vpn_uid = _vpn_user_id(update.effective_user.id)
    if context.bot_data.get("dry_run"):
        await _reply(
            update,
            "Режим проверки без сервера (BOT_DRY_RUN=1).\n\n"
            f"Запрос статуса к API не выполнялся.\n"
            f"Ваш userId был бы: {vpn_uid}",
        )
        return

    client: VPNProductClient = context.bot_data["vpn_client"]
    try:
        status, data = await client.issue_status(vpn_uid)
    except httpx.RequestError as e:
        await _reply(update, f"Сеть или сервер недоступны: {e}")
        return

    if status == 404:
        await _reply(
            update,
            "Активная подписка не найдена. Запросите выдачу через «Получить подписку».",
        )
        return
    if status != 200:
        await _reply(update, f"Ошибка API ({status}): {_api_error_message(data)}")
        return

    st = data.get("status", "?")
    sub_id = data.get("subscriptionId") or "—"
    applied = data.get("appliedTo3XUI")
    verr = (data.get("verifyError") or "").strip()
    lines = [
        f"Статус: {st}",
        f"ID подписки: {sub_id}",
    ]
    if applied is True:
        lines.append("3x-ui: подтверждено")
    elif applied is False:
        lines.append("3x-ui: не подтверждено")
    if verr:
        lines.append(f"Проверка: {verr}")
    await _reply(update, "\n".join(lines))


async def _do_history(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    vpn_uid = _vpn_user_id(update.effective_user.id)
    if context.bot_data.get("dry_run"):
        await _reply(
            update,
            "Режим проверки без сервера (BOT_DRY_RUN=1).\n\n"
            f"История из API не запрашивалась.\n"
            f"Ваш userId был бы: {vpn_uid}",
        )
        return

    client: VPNProductClient = context.bot_data["vpn_client"]
    try:
        status, data = await client.issue_history(vpn_uid, limit=10)
    except httpx.RequestError as e:
        await _reply(update, f"Сеть или сервер недоступны: {e}")
        return

    if status != 200:
        await _reply(update, f"Ошибка API ({status}): {_api_error_message(data)}")
        return

    items = data.get("items") or []
    if not isinstance(items, list) or not items:
        await _reply(update, "История выдач пуста.")
        return

    lines_out: list[str] = ["Последние выдачи:"]
    for i, it in enumerate(items[:10], 1):
        if not isinstance(it, dict):
            continue
        sid = it.get("subscriptionId", "—")
        th = it.get("tokenHint") or "—"
        src = it.get("source") or ""
        issued = it.get("issuedAt", "")
        exp = it.get("expiresAt", "")
        extra = f" ({src})" if src else ""
        lines_out.append(f"{i}. sub {sid}, hint {th}{extra}")
        lines_out.append(f"   выдано: {issued}  до: {exp}")

    await _reply(update, "\n".join(lines_out))


async def _reply(update: Update, text: str) -> None:
    if update.callback_query:
        await update.callback_query.edit_message_text(text, reply_markup=main_keyboard())
        return
    if update.message:
        await update.message.reply_text(text, reply_markup=main_keyboard())


async def cmd_subscribe(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    allowed: set[int] | None = context.bot_data.get("allowed_ids")
    if not is_allowed(update, allowed):
        await update.message.reply_text("Доступ ограничен.")
        return
    await _do_issue(update, context)


async def cmd_status(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    allowed: set[int] | None = context.bot_data.get("allowed_ids")
    if not is_allowed(update, allowed):
        await update.message.reply_text("Доступ ограничен.")
        return
    await _do_status(update, context)


async def cmd_history(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    allowed: set[int] | None = context.bot_data.get("allowed_ids")
    if not is_allowed(update, allowed):
        await update.message.reply_text("Доступ ограничен.")
        return
    await _do_history(update, context)


async def on_callback(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    q = update.callback_query
    if not q:
        return
    allowed: set[int] | None = context.bot_data.get("allowed_ids")
    if not is_allowed(update, allowed):
        await q.answer("Доступ ограничен.", show_alert=True)
        return
    await q.answer()
    data = (q.data or "").strip()
    if data == "issue":
        await _do_issue(update, context)
    elif data == "status":
        await _do_status(update, context)
    elif data == "history":
        await _do_history(update, context)


def main() -> None:
    _setup_loki_logging()
    token = os.environ.get("TELEGRAM_BOT_TOKEN", "").strip()
    base = os.environ.get("VPN_PRODUCT_BASE_URL", "").strip()
    api_token = os.environ.get("VPN_PRODUCT_API_TOKEN", "").strip()
    dry_run = _dry_run_enabled()

    if not token:
        raise SystemExit("TELEGRAM_BOT_TOKEN is required")
    if not dry_run:
        if not base:
            raise SystemExit(
                "VPN_PRODUCT_BASE_URL is required (or set BOT_DRY_RUN=1 for a Telegram-only test)",
            )
        if not api_token:
            raise SystemExit(
                "VPN_PRODUCT_API_TOKEN is required (or set BOT_DRY_RUN=1 for a Telegram-only test)",
            )

    allowed = _parse_allowed_ids(os.environ.get("ALLOWED_TELEGRAM_IDS"))
    profile_ids = _parse_profile_ids(os.environ.get("VPN_PROFILE_IDS"))

    app = (
        Application.builder()
        .token(token)
        .build()
    )
    app.bot_data["dry_run"] = dry_run
    if dry_run:
        app.bot_data["vpn_client"] = None
        logger.info("BOT_DRY_RUN enabled: no HTTP calls to vpn-productd")
    else:
        app.bot_data["vpn_client"] = VPNProductClient(base, api_token)
    app.bot_data["allowed_ids"] = allowed
    app.bot_data["profile_ids"] = profile_ids

    app.add_handler(CommandHandler("start", cmd_start))
    app.add_handler(CommandHandler("help", cmd_help))
    app.add_handler(CommandHandler("subscribe", cmd_subscribe))
    app.add_handler(CommandHandler("status", cmd_status))
    app.add_handler(CommandHandler("history", cmd_history))
    app.add_handler(CallbackQueryHandler(on_callback))

    logger.info("starting polling…")
    app.run_polling(allowed_updates=Update.ALL_TYPES)


if __name__ == "__main__":
    main()
