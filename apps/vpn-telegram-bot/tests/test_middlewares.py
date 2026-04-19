"""Middleware tests.

For `AuthMiddleware` we exercise the branches that don't require aiogram's
real `Message`/`CallbackQuery` `isinstance` dispatch — those are covered by
the handler-level tests via the same routing contract.
"""
from __future__ import annotations

from typing import Any

from tests._stubs import FakeUser
from vpn_bot.middlewares.throttling import ThrottlingMiddleware


class TestThrottling:
    async def test_passes_when_no_user(self) -> None:
        mw = ThrottlingMiddleware()
        called: list[int] = []

        async def _h(e: Any, d: Any) -> str:
            called.append(1)
            return "ok"

        class _E:
            from_user = None

        res = await mw(_h, _E(), {})
        assert res == "ok"
        assert called == [1]

    async def test_skips_when_called_too_fast(self) -> None:
        from vpn_bot.middlewares import throttling

        throttling._last.clear()
        mw = ThrottlingMiddleware()
        calls: list[int] = []

        async def _h(e: Any, d: Any) -> str:
            calls.append(1)
            return "ok"

        class _E:
            from_user = FakeUser(id=123)

        r1 = await mw(_h, _E(), {})
        r2 = await mw(_h, _E(), {})
        assert r1 == "ok"
        assert r2 is None
        assert calls == [1]


class TestAuthEventUser:
    def test_event_user_message(self) -> None:
        from aiogram.types import Chat, Message, User

        from vpn_bot.middlewares.auth import _event_user

        msg = Message.model_construct(
            message_id=1,
            chat=Chat(id=1, type="private"),
            from_user=User(id=42, is_bot=False, first_name="x"),
            date=0,
        )
        u = _event_user(msg)
        assert u is not None
        assert u.id == 42

    def test_event_user_unknown(self) -> None:
        from vpn_bot.middlewares.auth import _event_user

        class _Other:
            pass

        assert _event_user(_Other()) is None
