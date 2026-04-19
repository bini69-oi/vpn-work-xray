'use strict';

const express = require('express');
const cors = require('cors');
const path = require('path');
const { loadConfig } = require('./config');
const { validateTelegramData } = require('./telegram-init-data');
const { createAuthMiddleware } = require('./middleware/telegram-user');
const { maybeStartNodeTelegramBot } = require('./optional-polling-bot');
const { registerApiRoutes } = require('./routes/api');

// Optional second bot (polling). Default: off — use apps/vpn-telegram-bot (Python, aiogram) as primary.
function createApp(cfg) {
  const app = express();
  const webappDir = path.join(__dirname, '..', 'webapp');

  app.use(cors());
  app.use(express.json());
  app.use(express.static(webappDir));

  const authMiddleware = createAuthMiddleware(cfg.botToken, validateTelegramData);
  registerApiRoutes(app, {
    authMiddleware,
    vpnApiUrl: cfg.vpnApiUrl,
    vpnAdminToken: cfg.vpnAdminToken,
  });

  app.get('*', (_req, res) => {
    res.sendFile(path.join(webappDir, 'index.html'));
  });

  return app;
}

function start() {
  const cfg = loadConfig();
  maybeStartNodeTelegramBot({ botToken: cfg.botToken, webappUrl: cfg.webappUrl });
  const app = createApp(cfg);
  app.listen(cfg.port, () => {
    console.log(`🌐 Сервер запущен на порту ${cfg.port}`);
    console.log(`📱 Mini App URL: ${cfg.webappUrl}`);
  });
}

module.exports = { start, createApp, loadConfig };
