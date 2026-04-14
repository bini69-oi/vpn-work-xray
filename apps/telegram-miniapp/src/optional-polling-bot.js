'use strict';

/**
 * @param {{ botToken: string, webappUrl: string }} cfg
 */
function maybeStartNodeTelegramBot(cfg) {
  const enabled = String(process.env.START_TELEGRAM_BOT_POLLING || '').trim() === '1';
  if (!enabled) {
    console.log(
      '🤖 Telegram polling disabled (set START_TELEGRAM_BOT_POLLING=1 to enable node-telegram-bot-api)',
    );
    return;
  }

  const TelegramBot = require('node-telegram-bot-api');
  const bot = new TelegramBot(cfg.botToken, { polling: true });

  bot.onText(/\/start/, (msg) => {
    const chatId = msg.chat.id;
    const firstName = msg.from.first_name || 'друг';

    bot.sendMessage(
      chatId,
      `Привет, ${firstName}! 👋\n\nЯ — бот для управления VPN-подпиской.\nНажми кнопку ниже, чтобы открыть приложение.`,
      {
        reply_markup: {
          inline_keyboard: [
            [
              {
                text: '🔐 Открыть VPN',
                web_app: { url: cfg.webappUrl },
              },
            ],
          ],
        },
      },
    );
  });

  bot.onText(/\/help/, (msg) => {
    bot.sendMessage(
      msg.chat.id,
      [
        '📖 *Команды:*',
        '/start — открыть приложение',
        '/help — эта справка',
        '',
        'Всё управление через Mini App — нажми кнопку «Открыть VPN».',
      ].join('\n'),
      { parse_mode: 'Markdown' },
    );
  });

  console.log('🤖 Telegram polling bot enabled (START_TELEGRAM_BOT_POLLING=1)');
}

module.exports = { maybeStartNodeTelegramBot };
