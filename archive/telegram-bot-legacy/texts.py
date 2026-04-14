from __future__ import annotations


START_TEXT = (
    "Привет! Я бот VPN Product.\n\n"
    "Я умею выдавать подписку, показывать статус, ссылки для подключения и принимать оплату."
)

HELP_TEXT = (
    "Команды:\n"
    "/subscribe — получить подписку\n"
    "/status — статус и трафик\n"
    "/links — ссылки для подключения\n"
    "/renew — продлить на 30 дней\n"
    "/pay — оплатить подписку\n"
    "/history — история выдач\n"
)

ACCESS_DENIED = "Доступ ограничен."
API_UNAVAILABLE = "Сеть или сервер недоступны. Попробуйте позже."

NO_SUBSCRIPTION = "Подписка не найдена. Нажмите «🔑 Подписка» или используйте /subscribe."
RENEW_OK = "Подписка продлена."

PAYMENT_MANUAL_INSTRUCTIONS_PREFIX = "Оплата вручную:\n\n"
PAYMENT_MANUAL_NEED_SCREENSHOT = "Пришлите, пожалуйста, скриншот/чек оплаты одним сообщением."
PAYMENT_SENT_TO_ADMINS = "Спасибо! Запрос на проверку отправлен администратору."

