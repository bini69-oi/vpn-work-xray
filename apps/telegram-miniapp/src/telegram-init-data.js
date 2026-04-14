'use strict';

const crypto = require('crypto');

/**
 * @param {string|undefined} initData
 * @param {string} botToken
 * @returns {object|null} Telegram user object or null
 */
function validateTelegramData(initData, botToken) {
  if (!initData) return null;

  try {
    const params = new URLSearchParams(initData);
    const hash = params.get('hash');
    if (!hash) return null;

    const authDateRaw = params.get('auth_date');
    const authDate = Number(authDateRaw);
    if (!Number.isFinite(authDate)) return null;
    const ageSec = Math.abs(Date.now() / 1000 - authDate);
    if (ageSec > 24 * 3600) return null;

    params.delete('hash');
    const entries = [...params.entries()].sort(([a], [b]) => a.localeCompare(b));
    const dataCheckString = entries.map(([k, v]) => `${k}=${v}`).join('\n');

    const secretKey = crypto.createHmac('sha256', 'WebAppData').update(botToken).digest();

    const checkHash = crypto.createHmac('sha256', secretKey).update(dataCheckString).digest('hex');

    if (checkHash !== hash) return null;

    const userStr = params.get('user');
    if (!userStr) return null;

    return JSON.parse(userStr);
  } catch {
    return null;
  }
}

module.exports = { validateTelegramData };
