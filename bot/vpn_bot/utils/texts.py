from __future__ import annotations

from vpn_bot.utils.formatting import PRICING, days_left, format_date_ru


def access_denied() -> str:
    return "⛔ <b>Доступ ограничен</b>\n\n<i>Этот бот доступен только приглашённым пользователям.</i>"


def banned() -> str:
    return "🚫 <b>Аккаунт заблокирован</b>\n\nСвяжитесь с поддержкой, если это ошибка."


def service_unavailable() -> str:
    return (
        "⚠️ <b>Сервис временно недоступен</b>\n\n"
        "Не настроен доступ к API или сервер не отвечает. Попробуйте позже."
    )


def welcome_text() -> str:
    return (
        "🛡 <b>Добро пожаловать в VPN-32</b>\n"
        "\n"
        "Быстрый, безопасный и анонимный доступ\n"
        "к любым сайтам без ограничений.\n"
        "\n"
        "━━━━━━━━━━━━━━━\n"
        "\n"
        "⚡ Скорость до <b>150 Мбит/с</b>\n"
        "🌍 <b>Серверы в Нидерландах и Казахстане</b>\n"
        "🔐 <b>Трафик не логируется</b>\n"
        "📱 Работает на всех устройствах\n"
        "\n"
        "━━━━━━━━━━━━━━━\n"
        "\n"
        "Стоимость 90₽ в месяц"
    )


def main_reply_hint() -> str:
    return "⌨️ Используй кнопки меню внизу экрана."


def about_vpn_short() -> str:
    return (
        "📋 <b>VPN-32</b>\n"
        "\n"
        "Обход блокировок, без логов трафика, серверы в Европе и Казахстане. "
        "Оформить доступ — кнопка <b>«💎 Оплатить подписку»</b>.\n"
        "\n"
        "После оплаты в меню появится <b>«🛡 Мой VPN»</b> — профиль и подключение через HApp."
    )


def profile_text(
    *,
    status_label: str,
    plan_label: str,
    expires_iso: str | None,
    traffic_used: str,
    traffic_limit: str,
    devices_line: str,
) -> str:
    dl = days_left(expires_iso)
    return (
        "🛡 <b>Мой VPN</b>\n"
        "\n"
        "┌─ Подписка\n"
        f"│  Статус:  {status_label}\n"
        f"│  Тариф:   {plan_label}\n"
        f"│  До:      📅 <code>{format_date_ru(expires_iso)}</code>\n"
        f"│  Осталось: <b>{dl} дн.</b>\n"
        "└─\n"
        "\n"
        "┌─ Трафик\n"
        f"│  📊 Использовано: <b>{traffic_used}</b>\n"
        f"│  📈 Лимит: <b>{traffic_limit}</b>\n"
        "└─\n"
        "\n"
        "┌─ Устройства\n"
        f"│  {devices_line}\n"
        "└─"
    )


def no_subscription_text() -> str:
    return (
        "📭 <b>Подписка не найдена</b>\n"
        "\n"
        "Нажми внизу <b>«💎 Оплатить подписку»</b> или запроси выдачу у администратора."
    )


def pricing_text() -> str:
    lines = [
        "💎 <b>Выбери тариф</b>",
        "",
        "Чем больше срок — тем выгоднее!",
        "",
        "━━━━━━━━━━━━━━━",
        "",
    ]
    for m in (1, 2, 3, 6, 12):
        p = PRICING[m]
        disc = p["discount"]
        suffix = f' <i>(скидка {disc}%)</i>' if disc else ""
        mark = "🔥 " if m == 6 else "📦 "
        lines.append(f'{mark}<b>{p["months"]} мес.</b> — <b>{p["price"]} ₽</b>{suffix}')
    lines.extend(
        [
            "",
            "━━━━━━━━━━━━━━━",
            "",
            "💡 <i>Самый популярный — 6 месяцев</i>",
        ]
    )
    return "\n".join(lines)


def menu_updated_notice() -> str:
    return "⌨️ <b>Меню обновлено.</b>\n\nНиже актуальные кнопки — пользуйся ими вместо старых."


def pay_method_text(months: int, price: int) -> str:
    return (
        f"💎 <b>Оплата: {months} мес.</b> — <b>{price} ₽</b>\n"
        "\n"
        "Дальше — оплата по <b>СБП</b>: откроется страница оплаты (если задана ссылка) и кнопка подтверждения. "
        "Если настроена оплата картой через Telegram, появится отдельная кнопка."
    )


def sbp_payment_page_text(months: int, price: int, has_pay_url: bool) -> str:
    url_hint = (
        "Нажми <b>«Перейти к оплате»</b> — откроется страница оплаты."
        if has_pay_url
        else "<i>Ссылка на оплату не настроена (SBP_PAY_URL) — кнопка покажет подсказку.</i>"
    )
    return (
        f"💠 <b>Оплата по СБП</b> — <b>{months} мес.</b>, <b>{price} ₽</b>\n"
        "\n"
        f"{url_hint}\n"
        "\n"
        "После оплаты вернись в бот и нажми <b>«✅ Я оплатил»</b> — статус проверится "
        "(сейчас возможна <b>заглушка</b>, см. PAYMENT_STUB в .env).\n"
    )


def sbp_reply_menu_hint() -> str:
    return "⌨️ <b>Главное меню</b> — снова доступно внизу экрана."


def stub_payment_ok() -> str:
    return (
        "✅ <b>Заглушка: платёж зачислен</b>\n\n"
        "<i>Режим PAYMENT_STUB=1. Подключи реальную проверку СБП или выключи заглушку.</i>"
    )


def stub_payment_fail() -> str:
    return (
        "❌ <b>Заглушка: платёж не прошёл</b>\n\n"
        "<i>Режим PAYMENT_STUB=1, результат PAYMENT_STUB_RESULT=fail. "
        "Или оплата ещё не дошла — попробуй позже.</i>"
    )


def manual_pay_instructions(details: str) -> str:
    base = "💠 <b>Оплата по СБП</b>\n\nПереведи сумму по реквизитам ниже.\n\n━━━━━━━━━━━━━━━\n\n"
    if details.strip():
        return base + details.strip()
    return base + "<i>Реквизиты не настроены. Напишите в поддержку.</i>"


def crypto_instructions(amount_rub: int, wallet: str) -> str:
    w = wallet.strip() or "— (не настроен)"
    return (
        "🌐 <b>Оплата в USDT (TRC-20)</b>\n"
        "\n"
        "━━━━━━━━━━━━━━━\n"
        "\n"
        f"Сумма к оплате (ориентир): <b>{amount_rub} ₽</b> эквивалентом в USDT по курсу обменника.\n"
        "\n"
        "Адрес кошелька:\n"
        f"<code>{w}</code>\n"
        "\n"
        "После перевода нажми <b>«✅ Я оплатил»</b>."
    )


def payment_pending_admin(user_label: str, months: int, amount_rub: int, pid: int) -> str:
    return (
        "💰 <b>Запрос на оплату</b>\n"
        "\n"
        f"Пользователь: {user_label}\n"
        f"Тариф: <b>{months} мес.</b>\n"
        f"Сумма: <b>{amount_rub} ₽</b>\n"
        f"<code>payment_id={pid}</code>"
    )


def payment_confirmed_user(expires_hint: str) -> str:
    return (
        "✅ <b>Оплата подтверждена</b>\n"
        "\n"
        f"Подписка обновлена. {expires_hint}\n"
        "\n"
        "Открой <b>«🛡 Мой VPN»</b> → <b>«📲 Подключить (HApp)»</b>."
    )


def referral_text(bot_username: str, referrer_id: int, invited: int, paid: int, bonus_per_friend: int) -> str:
    link = f"https://t.me/{bot_username}?start=ref_{referrer_id}"
    earned = paid * bonus_per_friend
    return (
        "🎁 <b>Реферальная программа</b>\n"
        "\n"
        "Приглашай друзей и получай бонусы!\n"
        "\n"
        "━━━━━━━━━━━━━━━\n"
        "\n"
        "🔗 Твоя ссылка (если нужна вручную):\n"
        f"<code>{link}</code>\n"
        "\n"
        "✉️ Нажми <b>«📨 Отправить сообщение»</b> — придёт <b>одно</b> сообщение с пометкой сверху "
        "«Отправь другу» и кнопкой. Перешли его другу (⋯ → Переслать).\n"
        "\n"
        "━━━━━━━━━━━━━━━\n"
        "\n"
        "📊 <b>Твоя статистика:</b>\n"
        "\n"
        f"👥 Приглашено: <b>{invited}</b>\n"
        f"✅ Оплатили:  <b>{paid}</b>\n"
        f"🎁 Уже начислено бонусом: <b>+{earned} дн.</b>\n"
        "\n"
        "━━━━━━━━━━━━━━━\n"
        "\n"
        f"💡 За каждого оплатившего друга — <b>+{bonus_per_friend} дн.</b> к подписке."
    )


def referral_invite_friend_card() -> str:
    """Одно сообщение для пересылки: шапка + текст + кнопка (URL в кнопке)."""
    return (
        "📨 <b>Отправь другу</b>\n"
        "\n"
        "━━━━━━━━━━━━━━━\n"
        "\n"
        "🛡 <b>VPN-32 — обход блокировок</b>\n"
        "\n"
        "Тебя пригласили попробовать наш сервис: стабильная скорость, "
        "серверы в Европе и Казахстане, трафик не логируем.\n"
        "\n"
        "Всего <b>90 ₽</b> в месяц.\n"
        "\n"
        "━━━━━━━━━━━━━━━\n"
        "\n"
        "👇 <b>Нажми кнопку «Открыть VPN-32» ниже</b> — так приглашение "
        "засчитается твоему другу."
    )


def happ_connect_instructions() -> str:
    return (
        "📲 <b>Подключение через HApp</b>\n"
        "\n"
        "1) Установи приложение <b>HApp</b> (Happ — Proxy Utility) из App Store или Google Play.\n"
        "2) Нажми кнопку <b>«Открыть в HApp»</b> ниже — подписка подтянется в приложение.\n"
        "3) В HApp выбери профиль и включи подключение.\n"
        "\n"
        "<i>Если кнопка не открывает приложение, обнови HApp или напиши в поддержку.</i>"
    )


def renew_hint_text() -> str:
    return (
        "🔄 <b>Продление</b>\n"
        "\n"
        "Выбери срок подписки ниже или открой в меню <b>«💎 Оплатить подписку»</b>."
    )


def referral_friend_joined() -> str:
    return "🎉 <b>Друг перешёл по твоей ссылке!</b>"


def referral_friend_paid(bonus: int) -> str:
    return f"🎁 <b>Друг оплатил подписку!</b>\n\nТебе начислено <b>+{bonus} дн.</b>"


def help_menu_text() -> str:
    return "💬 <b>Помощь</b>\n\nВыбери тему ниже."


def faq_connect() -> str:
    return (
        "📱 <b>Как подключиться?</b>\n"
        "\n"
        "1) Скачайте приложения Happ в App Store или Google Play</b>.\n"
        "2) Открой <b>«🛡 Мой VPN»</b> → <b>«📲 Подключить (HApp)»</b> и следуй подсказкам.\n"
        "3) Включи VPN в HApp и проверь доступ к сайтам.\n"
    )


def faq_not_working() -> str:
    return (
        "🔄 <b>Не работает VPN</b>\n"
        "\n"
        "• Проверь дату подписки в <b>«🛡 Мой VPN»</b>.\n"
        "• Обнови подписку в клиенте (pull / update).\n"
        "• Смени сеть (Wi‑Fi ↔ LTE).\n"
        "• Напиши в поддержку с описанием ошибки.\n"
    )


def faq_payment() -> str:
    return (
        "💳 <b>Вопросы по оплате</b>\n"
        "\n"
        "Выбери срок → <b>«Оплатить по СБП»</b> → страница оплаты (если настроена ссылка) → "
        "<b>«✅ Я оплатил»</b>. При отключённой заглушке заявку проверит администратор.\n"
    )


def admin_only() -> str:
    return "🔒 <b>Только для администраторов</b>"


def admin_menu_text() -> str:
    return "⚙️ <b>Админ-панель</b>\n\nВыбери действие:"


def broadcast_ask_text() -> str:
    return "📢 <b>Рассылка</b>\n\nОтправь текст сообщения (поддерживается HTML)."


def broadcast_preview(html: str) -> str:
    return "👁 <b>Превью рассылки</b>\n\n━━━━━━━━━━━━━━━\n\n" + html


def broadcast_result(sent: int, failed: int, total: int) -> str:
    return (
        "✅ <b>Рассылка завершена</b>\n"
        "\n"
        f"Доставлено: <b>{sent}</b>\n"
        f"Ошибок: <b>{failed}</b>\n"
        f"Всего: <b>{total}</b>\n"
    )
