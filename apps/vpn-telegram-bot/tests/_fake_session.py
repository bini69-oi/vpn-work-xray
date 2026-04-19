"""Fake aiohttp.ClientSession for unit tests.

The production code uses a narrow subset of `aiohttp.ClientSession`:

    async with session.request(method, url, headers=..., params=..., json=...) as resp:
        text = await resp.text()
        resp.status

`FakeSession` duck-types exactly that contract so we don't need real HTTP or
the `aioresponses`/`aiohttp-mock` optional dependencies.
"""
from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass, field
from typing import Any


@dataclass
class RecordedCall:
    method: str
    url: str
    headers: dict[str, str] | None = None
    params: dict[str, str] | None = None
    json: Any | None = None


class FakeResponse:
    def __init__(self, status: int, body: str | bytes = "") -> None:
        self.status = status
        self._body = body if isinstance(body, str) else body.decode("utf-8", "replace")

    async def text(self) -> str:
        return self._body

    async def __aenter__(self) -> FakeResponse:
        return self

    async def __aexit__(self, *_: Any) -> None:
        return None


# A route matcher: given a recorded call, return `FakeResponse` or None to skip.
Matcher = Callable[[RecordedCall], FakeResponse | None]


@dataclass
class FakeSession:
    """Minimal async HTTP session stub."""

    matchers: list[Matcher] = field(default_factory=list)
    calls: list[RecordedCall] = field(default_factory=list)
    default_status: int = 404
    default_body: str = '{"message":"no route"}'

    def request(
        self,
        method: str,
        url: str,
        *,
        headers: dict[str, str] | None = None,
        params: dict[str, str] | None = None,
        json: Any | None = None,
    ) -> FakeResponse:
        call = RecordedCall(method=method, url=url, headers=headers, params=params, json=json)
        self.calls.append(call)
        for m in self.matchers:
            resp = m(call)
            if resp is not None:
                return resp
        return FakeResponse(self.default_status, self.default_body)

    def post(
        self,
        url: str,
        *,
        headers: dict[str, str] | None = None,
        params: dict[str, str] | None = None,
        json: Any | None = None,
    ) -> FakeResponse:
        return self.request("POST", url, headers=headers, params=params, json=json)

    def get(
        self,
        url: str,
        *,
        headers: dict[str, str] | None = None,
        params: dict[str, str] | None = None,
    ) -> FakeResponse:
        return self.request("GET", url, headers=headers, params=params)

    def on(
        self,
        method: str,
        path_suffix: str,
        status: int = 200,
        body: str = "{}",
    ) -> None:
        """Register a route that matches on HTTP method + URL suffix."""

        def _matcher(call: RecordedCall) -> FakeResponse | None:
            if call.method.upper() != method.upper():
                return None
            if not call.url.endswith(path_suffix):
                return None
            return FakeResponse(status, body)

        self.matchers.append(_matcher)

    def on_dynamic(self, fn: Matcher) -> None:
        self.matchers.append(fn)

    def last_call(self) -> RecordedCall:
        assert self.calls, "no recorded calls"
        return self.calls[-1]

    def calls_for(self, method: str, path_suffix: str) -> list[RecordedCall]:
        return [
            c
            for c in self.calls
            if c.method.upper() == method.upper() and c.url.endswith(path_suffix)
        ]
