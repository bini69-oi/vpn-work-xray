from __future__ import annotations

import json
from typing import Any

import httpx


class VPNProductClient:
    """Async HTTP client for vpn-productd API."""

    def __init__(self, base_url: str, api_token: str, timeout_seconds: float = 60.0) -> None:
        self._base = base_url.rstrip("/")
        self._headers = {"Authorization": f"Bearer {api_token}"}
        self._timeout = httpx.Timeout(timeout_seconds)

    async def _get(self, path: str, params: dict[str, str] | None = None) -> tuple[int, dict[str, Any]]:
        async with httpx.AsyncClient(timeout=self._timeout) as client:
            r = await client.get(f"{self._base}{path}", params=params, headers=dict(self._headers))
        return _decode_json(r)

    async def _post(self, path: str, body: dict[str, Any] | None = None) -> tuple[int, dict[str, Any]]:
        async with httpx.AsyncClient(timeout=self._timeout) as client:
            r = await client.post(f"{self._base}{path}", json=body or {}, headers=dict(self._headers))
        return _decode_json(r)

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
        async with httpx.AsyncClient(timeout=self._timeout) as client:
            r = await client.post(f"{self._base}/v1/issue/link", json=body, headers=headers)
        return _decode_json(r)

    async def issue_status(self, user_id: str) -> tuple[int, dict[str, Any]]:
        return await self._get("/v1/issue/status", params={"userId": user_id})

    async def issue_history(self, user_id: str, limit: int = 10) -> tuple[int, dict[str, Any]]:
        return await self._get("/v1/issue/history", params={"userId": user_id, "limit": str(limit)})

    async def lifecycle(self, user_id: str, action: str, days: int | None = None) -> tuple[int, dict[str, Any]]:
        body: dict[str, Any] = {"userId": user_id, "action": action}
        if days is not None:
            body["days"] = int(days)
        return await self._post("/v1/subscriptions/lifecycle", body=body)

    async def get_delivery_links(self, profile_id: str) -> tuple[int, dict[str, Any]]:
        return await self._get("/v1/delivery/links", params={"profileId": profile_id})

    async def get_account(self, profile_id: str) -> tuple[int, dict[str, Any]]:
        return await self._get("/v1/account", params={"profileId": profile_id})

    async def get_profile_stats(self) -> tuple[int, dict[str, Any]]:
        return await self._get("/v1/stats/profiles")

    async def get_profiles(self) -> tuple[int, dict[str, Any]]:
        return await self._get("/v1/profiles")

    async def get_health(self) -> tuple[int, dict[str, Any]]:
        return await self._get("/v1/health")

    async def get_subscription(self, subscription_id: str) -> tuple[int, dict[str, Any]]:
        return await self._get(f"/v1/subscriptions/{subscription_id}")


def _decode_json(resp: httpx.Response) -> tuple[int, dict[str, Any]]:
    try:
        data = resp.json()
    except json.JSONDecodeError:
        data = {"error": resp.text or f"HTTP {resp.status_code}"}
    if not isinstance(data, dict):
        data = {"error": str(data)}
    return resp.status_code, data

