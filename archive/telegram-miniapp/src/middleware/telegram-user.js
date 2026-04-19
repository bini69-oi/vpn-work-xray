'use strict';

/**
 * @param {string} botToken
 * @param {(initData: string|undefined, botToken: string) => object|null} validateTelegramData
 */
function createAuthMiddleware(botToken, validateTelegramData) {
  return function authMiddleware(req, res, next) {
    const initData = req.headers['x-telegram-init-data'];
    const user = validateTelegramData(initData, botToken);

    if (!user) {
      return res.status(401).json({ error: 'Unauthorized: invalid Telegram data' });
    }

    req.tgUser = user;
    req.vpnUserId = `tg_${user.id}`;
    next();
  };
}

module.exports = { createAuthMiddleware };
