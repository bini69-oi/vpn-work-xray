'use strict';

const { vpnApi } = require('../vpn-product-client');

/** Register `/api/*` routes (expects `express.json` and static already on `app`). */
function registerApiRoutes(app, deps) {
  const { authMiddleware, vpnApiUrl, vpnAdminToken } = deps;

  app.get('/api/status', authMiddleware, async (req, res) => {
    try {
      const result = await vpnApi(vpnApiUrl, vpnAdminToken, 'GET', `/admin/user/${req.vpnUserId}/status`);
      if (result.status < 200 || result.status >= 300) {
        return res.status(result.status >= 400 ? result.status : 502).json(result.data);
      }
      res.json({
        userId: req.vpnUserId,
        tgUser: {
          id: req.tgUser.id,
          firstName: req.tgUser.first_name,
          username: req.tgUser.username,
        },
        ...result.data,
      });
    } catch (err) {
      console.error('Status error:', err);
      res.status(502).json({ error: 'VPN API unavailable' });
    }
  });

  app.post('/api/subscribe', authMiddleware, async (req, res) => {
    try {
      const result = await vpnApi(vpnApiUrl, vpnAdminToken, 'POST', '/admin/issue/link', {
        userId: req.vpnUserId,
        source: 'telegram_miniapp',
      });
      if (result.status < 200 || result.status >= 300) {
        return res.status(result.status >= 400 ? result.status : 502).json(result.data);
      }
      res.json(result.data);
    } catch (err) {
      console.error('Subscribe error:', err);
      res.status(502).json({ error: 'VPN API unavailable' });
    }
  });

  app.get('/api/profile', authMiddleware, async (req, res) => {
    try {
      const result = await vpnApi(vpnApiUrl, vpnAdminToken, 'GET', `/admin/user/${req.vpnUserId}/profile`);
      if (result.status < 200 || result.status >= 300) {
        return res.status(result.status >= 400 ? result.status : 502).json(result.data);
      }
      res.json({
        userId: req.vpnUserId,
        tgUser: {
          id: req.tgUser.id,
          firstName: req.tgUser.first_name,
          lastName: req.tgUser.last_name,
          username: req.tgUser.username,
        },
        ...result.data,
      });
    } catch (err) {
      console.error('Profile error:', err);
      res.status(502).json({ error: 'VPN API unavailable' });
    }
  });

  app.get('/api/health', (_req, res) => {
    res.json({ ok: true, time: new Date().toISOString() });
  });
}

module.exports = { registerApiRoutes };
