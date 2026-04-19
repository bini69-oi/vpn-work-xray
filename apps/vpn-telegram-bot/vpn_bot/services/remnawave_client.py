from __future__ import annotations

import json
import logging
import re
from datetime import UTC, datetime, timedelta
from typing import Any

import aiohttp

log = logging.getLogger(__name__)


def _unwrap_response(data: Any) -> Any:
    if isinstance(data, dict) and isinstance(data.get("response"), dict):
        return data["response"]
    return data


class ApiError(Exception):
    def __init__(self, status: int, body: str) -> None:
        super().__init__(f"HTTP {status}: {body[:500]}")
        self.status = status
        self.body = body


def _normalize_api_root(panel_url: str) -> str:
    base = (panel_url or "").strip().rstrip("/")
    if not base:
        return ""
    if base.endswith("/api"):
        return base
    return base + "/api"


def _parse_telegram_user_id(vpn_user_id: str) -> int | None:
    s = (vpn_user_id or "").strip()
    m = re.fullmatch(r"tg_(\d+)", s, flags=re.IGNORECASE)
    if m:
        return int(m.group(1))
    if s.isdigit():
        return int(s)
    return None


def _username_for_telegram(telegram_id: int) -> str:
    u = f"tg{telegram_id}"
    if len(u) < 3:
        u = f"tg{telegram_id:03d}"
    return u[:36]


class RemnawaveApiClient:
    """HTTP-клиент к Remnawave Panel REST API (префикс /api, см. python-sdk)."""

    def __init__(
        self,
        session: aiohttp.ClientSession,
        panel_url: str,
        api_token: str,
        *,
        caddy_token: str = "",
        internal_squad_uuids: list[str] | None = None,
    ) -> None:
        self._session = session
        self._root = _normalize_api_root(panel_url)
        tok = (api_token or "").strip()
        self._headers: dict[str, str] = {
            "Authorization": tok if tok.lower().startswith("bearer ") else f"Bearer {tok}",
            "Content-Type": "application/json",
        }
        ct = (caddy_token or "").strip()
        if ct:
            self._headers["X-Api-Key"] = ct
        if panel_url.strip().lower().startswith("http://"):
            self._headers.setdefault("x-forwarded-proto", "https")
            self._headers.setdefault("x-forwarded-for", "127.0.0.1")
        self._squads = [s.strip() for s in (internal_squad_uuids or []) if s.strip()]

    def _url(self, path: str) -> str:
        p = path if path.startswith("/") else "/" + path
        return f"{self._root}{p}"

    async def _request(
        self,
        method: str,
        path: str,
        *,
        params: dict[str, str] | None = None,
        json_body: dict[str, Any] | None = None,
    ) -> tuple[int, Any]:
        url = self._url(path)
        try:
            async with self._session.request(
                method,
                url,
                headers=self._headers,
                params=params,
                json=json_body,
            ) as resp:
                text = await resp.text()
                try:
                    data = json.loads(text) if text else {}
                except json.JSONDecodeError:
                    data = {"raw": text}
                if not isinstance(data, (dict, list)):
                    data = {"value": data}
                return resp.status, data
        except aiohttp.ClientError as e:
            log.warning("remnawave request failed: %s %s: %s", method, path, e)
            raise ApiError(0, str(e)) from e

    async def _get_user_list_by_telegram(self, telegram_id: int) -> tuple[int, list[dict[str, Any]]]:
        st, data = await self._request("GET", f"/users/by-telegram-id/{telegram_id}")
        if st == 404:
            return st, []
        unwrapped = _unwrap_response(data)
        if isinstance(unwrapped, dict) and str(unwrapped.get("uuid") or "").strip():
            return st, [unwrapped]
        if isinstance(unwrapped, list):
            return st, [x for x in unwrapped if isinstance(x, dict)]
        if isinstance(data, list):
            return st, [x for x in data if isinstance(x, dict)]
        if isinstance(data, dict):
            inner = data.get("users") or data.get("data")
            if isinstance(inner, list):
                return st, [x for x in inner if isinstance(x, dict)]
        return st, []

    async def _get_one_user(self, telegram_id: int) -> tuple[int, dict[str, Any] | None]:
        st, users = await self._get_user_list_by_telegram(telegram_id)
        if st != 200 or not users:
            return (404 if st == 200 else st), None
        return 200, users[0]

    async def issue_status(self, user_id: str) -> tuple[int, dict[str, Any]]:
        tid = _parse_telegram_user_id(user_id)
        if tid is None:
            return 400, {"message": "invalid userId"}
        st, u = await self._get_one_user(tid)
        if st != 200 or not u:
            return 404, {"message": "active subscription not found"}
        uid = str(u.get("uuid") or "").strip()
        if not uid:
            return 502, {"message": "remnawave user without uuid"}
        return 200, {
            "userId": user_id,
            "subscriptionId": uid,
            "status": "verified" if str(u.get("status", "")).upper() == "ACTIVE" else "issued",
        }

    async def issue_link(
        self,
        user_id: str,
        name: str,
        source: str,
        profile_ids: list[str] | None,
        idempotency_key: str | None,
    ) -> tuple[int, dict[str, Any]]:
        _ = name, source, profile_ids, idempotency_key
        tid = _parse_telegram_user_id(user_id)
        if tid is None:
            return 400, {"message": "invalid userId"}
        st0, existing = await self._get_one_user(tid)
        if st0 == 200 and existing:
            uid = str(existing.get("uuid") or "").strip()
            return 200, {
                "subscription": {"id": uid, "token": "", "userId": user_id},
                "url": str(existing.get("subscriptionUrl") or "").strip(),
                "days": 30,
                "appliedTo3xui": True,
                "applyError": "subscription renewed; existing token is unchanged (url is not re-issued)",
            }
        if not self._squads:
            return 503, {"message": "REMNAWAVE_INTERNAL_SQUAD_UUIDS is not configured"}
        exp = (datetime.now(UTC) + timedelta(days=30)).isoformat().replace("+00:00", "Z")
        body: dict[str, Any] = {
            "username": _username_for_telegram(tid),
            "expireAt": exp,
            "telegramId": tid,
            "status": "ACTIVE",
            "trafficLimitStrategy": "NO_RESET",
            "activeInternalSquads": self._squads,
        }
        st, raw = await self._request("POST", "/users", json_body=body)
        if st not in (200, 201):
            return st, raw if isinstance(raw, dict) else {"raw": raw}
        data = _unwrap_response(raw)
        if not isinstance(data, dict):
            return 502, {"message": "unexpected create user response"}
        uid = str(data.get("uuid") or "").strip()
        sub_url = str(data.get("subscriptionUrl") or data.get("subscription_url") or "").strip()
        return 200, {
            "subscription": {"id": uid, "token": "", "userId": user_id},
            "url": sub_url,
            "days": 30,
            "appliedTo3xui": True,
            "profileId": f"user-tg-{tid}",
        }

    async def lifecycle_renew(self, user_id: str, days: int) -> tuple[int, dict[str, Any]]:
        tid = _parse_telegram_user_id(user_id)
        if tid is None:
            return 400, {"message": "invalid userId"}
        st, u = await self._get_one_user(tid)
        if st != 200 or not u:
            return 404, {"message": "user not found"}
        uid = str(u.get("uuid") or "").strip()
        raw_exp = u.get("expireAt") or u.get("expire_at")
        try:
            if isinstance(raw_exp, str) and raw_exp.strip():
                exp_dt = datetime.fromisoformat(raw_exp.replace("Z", "+00:00"))
            else:
                exp_dt = datetime.now(UTC)
        except ValueError:
            exp_dt = datetime.now(UTC)
        if exp_dt.tzinfo is None:
            exp_dt = exp_dt.replace(tzinfo=UTC)
        new_exp = (max(exp_dt, datetime.now(UTC)) + timedelta(days=int(days))).isoformat().replace(
            "+00:00", "Z"
        )
        patch: dict[str, Any] = {"uuid": uid, "expireAt": new_exp}
        st2, data2 = await self._request("PATCH", "/users", json_body=patch)
        if st2 not in (200, 201):
            return st2, data2 if isinstance(data2, dict) else {"raw": data2}
        return 200, {"ok": True}

    async def get_subscription(self, subscription_id: str) -> tuple[int, dict[str, Any]]:
        sid = (subscription_id or "").strip()
        if not sid:
            return 400, {"message": "subscription id required"}
        st, raw = await self._request("GET", f"/users/{sid}")
        u = _unwrap_response(raw)
        if st != 200 or not isinstance(u, dict):
            return st, u if isinstance(u, dict) else {"message": "not found"}
        exp = u.get("expireAt") or u.get("expire_at")
        exp_s = exp if isinstance(exp, str) else None
        lim = int(u.get("trafficLimitBytes") or u.get("traffic_limit_bytes") or 0)
        used = 0.0
        ut = u.get("userTraffic") or u.get("user_traffic")
        if isinstance(ut, dict):
            used = float(ut.get("usedTrafficBytes") or ut.get("used_traffic_bytes") or 0)
        return 200, {
            "id": sid,
            "expiresAt": exp_s,
            "trafficLimitBytes": lim,
            "usedTrafficBytes": int(used),
        }

    async def get_delivery_links(self, profile_id: str) -> tuple[int, dict[str, Any]]:
        m = re.fullmatch(r"user-tg-(\d+)", (profile_id or "").strip(), flags=re.IGNORECASE)
        if not m:
            return 400, {"message": "invalid profile id"}
        tid = int(m.group(1))
        st, u = await self._get_one_user(tid)
        if st != 200 or not u:
            return 404, {"links": {}}
        sub_url = str(u.get("subscriptionUrl") or "").strip()
        if not sub_url:
            return 404, {"links": {}}
        return 200, {"links": {"subscription": sub_url}}

    async def get_health(self) -> tuple[int, dict[str, Any]]:
        return await self._request("GET", "/system/health")
