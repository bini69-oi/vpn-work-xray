'use strict';

function loadConfig() {
  const {
    BOT_TOKEN,
    VPN_API_URL = 'http://localhost:8080',
    VPN_ADMIN_TOKEN,
    WEBAPP_URL = 'http://localhost:3000',
    PORT = 3000,
  } = process.env;

  if (!BOT_TOKEN) {
    console.error('❌ BOT_TOKEN не задан в .env');
    process.exit(1);
  }
  if (!VPN_ADMIN_TOKEN) {
    console.error('❌ VPN_ADMIN_TOKEN не задан в .env');
    process.exit(1);
  }

  const port = Number(PORT);
  return {
    botToken: BOT_TOKEN,
    vpnApiUrl: VPN_API_URL,
    vpnAdminToken: VPN_ADMIN_TOKEN,
    webappUrl: WEBAPP_URL,
    port: Number.isFinite(port) ? port : 3000,
  };
}

module.exports = { loadConfig };
