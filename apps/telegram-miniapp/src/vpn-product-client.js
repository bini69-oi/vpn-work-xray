'use strict';

/**
 * @param {string} vpnApiUrl
 * @param {string} vpnAdminToken
 * @param {string} method
 * @param {string} endpoint
 * @param {object|undefined} body
 */
async function vpnApi(vpnApiUrl, vpnAdminToken, method, endpoint, body) {
  const url = `${vpnApiUrl}${endpoint}`;
  const opts = {
    method,
    headers: {
      Authorization: `Bearer ${vpnAdminToken}`,
      'Content-Type': 'application/json',
    },
  };
  if (body) opts.body = JSON.stringify(body);

  const resp = await fetch(url, opts);
  const text = await resp.text();

  let data;
  try {
    data = JSON.parse(text);
  } catch {
    data = { raw: text };
  }

  return { status: resp.status, data };
}

module.exports = { vpnApi };
