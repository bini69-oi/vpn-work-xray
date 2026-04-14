from __future__ import annotations

import json
import logging
from typing import Any

import aiohttp

log = logging.getLogger(__name__)


class ApiError(Exception):
    def __init__(self, status: int, body: str) -> None:
        super().__init__(f"HTTP {status}: {body[:500]}")
        self.status = status
        self.body = body


class VPNApiClient:
    """HTTP-клиент к vpn-productd (реальные пути /v1/...)."""

    def __init__(self, session: aiohttp.ClientSession, base_url: str, token: str) -> None:
        self._session = session
        self._base = base_url.rstrip("/")
        self._headers = {"Authorization": f"Bearer {token}"}

    async def _request(
        self,
        method: str,
        path: str,
        *,
        params: dict[str, str] | None = None,
        json_body: dict[str, Any] | None = None,
    ) -> tuple[int, Any]:
        url = f"{self._base}{path}"
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
                if not isinstance(data, dict):
                    data = {"value": data}
                return resp.status, data
        except aiohttp.ClientError as e:
            log.warning("api request failed: %s %s: %s", method, path, e)
            raise ApiError(0, str(e)) from e

    async def issue_status(self, user_id: str) -> tuple[int, dict[str, Any]]:
        return await self._request("GET", "/v1/issue/status", params={"userId": user_id})

    async def issue_link(
        self,
        user_id: str,
        name: str,
        source: str,
        profile_ids: list[str] | None,
        idempotency_key: str | None,
    ) -> tuple[int, dict[str, Any]]:
        body: dict[str, Any] = {"userId": user_id, "name": name, "source": source}
        if profile_ids:
            body["profileIds"] = profile_ids
        headers = dict(self._headers)
        if idempotency_key:
            headers["X-Idempotency-Key"] = idempotency_key
        url = f"{self._base}/v1/issue/link"
        async with self._session.post(url, headers=headers, json=body) as resp:
            text = await resp.text()
            try:
                data = json.loads(text) if text else {}
            except json.JSONDecodeError:
                data = {"raw": text}
            if not isinstance(data, dict):
                data = {"value": data}
            return resp.status, data

    async def lifecycle_renew(self, user_id: str, days: int) -> tuple[int, dict[str, Any]]:
        return await self._request(
            "POST",
            "/v1/subscriptions/lifecycle",
            json_body={"userId": user_id, "action": "renew", "days": int(days)},
        )

    async def get_subscription(self, subscription_id: str) -> tuple[int, dict[str, Any]]:
        return await self._request("GET", f"/v1/subscriptions/{subscription_id}")

    async def get_delivery_links(self, profile_id: str) -> tuple[int, dict[str, Any]]:
        return await self._request("GET", "/v1/delivery/links", params={"profileId": profile_id})

    async def get_health(self) -> tuple[int, dict[str, Any]]:
        return await self._request("GET", "/v1/health")

    async def get_profile_stats(self) -> tuple[int, dict[str, Any]]:
        return await self._request("GET", "/v1/stats/profiles")
