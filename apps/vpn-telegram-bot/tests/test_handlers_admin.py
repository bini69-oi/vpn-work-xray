"""Handler-level tests for the admin FSM (monitoring + broadcast).

We drive the actual handler coroutines from `vpn_bot.handlers.admin` with
hand-rolled Message/CallbackQuery/FSMContext stubs + an in-memory SQLite.
"""
from __future__ import annotations

import pytest

from tests._stubs import FakeBot, FakeCallbackQuery, FakeMessage, FakeUser, FSMData, make_memory_db
from vpn_bot.handlers import admin as A


class AdminBackend:
    def __init__(self) -> None:
        self.health_resp: tuple[int, dict] = (200, {"status": "ok", "uptime": 42})

    async def issue_status(self, user_id: str):
        return 200, {}

    async def issue_link(self, *a, **kw):
        return 200, {}

    async def lifecycle_renew(self, user_id: str, days: int):
        return 200, {}

    async def get_subscription(self, sid: str):
        return 200, {}

    async def get_delivery_links(self, pid: str):
        return 200, {}

    async def get_health(self):
        return self.health_resp


@pytest.fixture
def admin_user() -> FakeUser:
    return FakeUser(id=1, username="admin")


@pytest.fixture
def regular_user() -> FakeUser:
    return FakeUser(id=999, username="user")


@pytest.fixture(autouse=True)
def _admin_env(monkeypatch: pytest.MonkeyPatch) -> None:
    from vpn_bot.config import settings
    monkeypatch.setattr(settings, "admin_ids_csv", "1", raising=False)


class TestAdminCommand:
    async def test_admin_gets_menu(self, admin_user) -> None:
        msg = FakeMessage(text="/admin", from_user=admin_user)
        await A.cmd_admin(msg)
        assert any("Админ-панель" in a.text for a in msg.answers)

    async def test_non_admin_is_blocked(self, regular_user) -> None:
        msg = FakeMessage(text="/admin", from_user=regular_user)
        await A.cmd_admin(msg)
        assert any("администратор" in a.text.lower() for a in msg.answers)


class TestAdminMenu:
    async def test_admin_menu_edits_for_admin(self, admin_user) -> None:
        q = FakeCallbackQuery(data="admin_menu", from_user=admin_user)
        await A.cb_admin_menu(q)
        assert any("Админ-панель" in e.text for e in q.message.edits)

    async def test_admin_menu_ignored_for_non_admin(self, regular_user) -> None:
        q = FakeCallbackQuery(data="admin_menu", from_user=regular_user)
        await A.cb_admin_menu(q)
        assert q.message.edits == []


class TestMonitoringRefresh:
    async def test_refresh_renders_remnawave_title(self, admin_user) -> None:
        api = AdminBackend()
        q = FakeCallbackQuery(data="mon_refresh", from_user=admin_user)
        await A.cb_mon_refresh(q, api)
        assert q.message.edits
        assert "Remnawave Panel" in q.message.edits[0].text

    async def test_refresh_non_admin_no_edit(self, regular_user) -> None:
        api = AdminBackend()
        q = FakeCallbackQuery(data="mon_refresh", from_user=regular_user)
        await A.cb_mon_refresh(q, api)
        assert q.message.edits == []


class TestBroadcastFSM:
    async def test_non_admin_cannot_start(self, regular_user) -> None:
        state = FSMData()
        q = FakeCallbackQuery(data="bc_start", from_user=regular_user)
        await A.cb_bc_start(q, state)
        assert state.state is None
        assert q.answered and q.answered[0] == ("Нет прав", True)

    async def test_admin_can_start_and_preview(self, admin_user) -> None:
        state = FSMData()

        q1 = FakeCallbackQuery(data="bc_start", from_user=admin_user)
        await A.cb_bc_start(q1, state)
        assert state.state and "waiting_text" in state.state

        msg = FakeMessage(text="Привет всем", from_user=admin_user)
        await A.bc_capture(msg, state)
        assert state.state and "preview" in state.state
        assert any("Превью рассылки" in a.text for a in msg.answers)
        assert state.data["bc_text"] == "Привет всем"

    async def test_broadcast_cancel_clears_state(self, admin_user) -> None:
        state = FSMData(state="BroadcastStates:preview", data={"bc_text": "x"})
        q = FakeCallbackQuery(data="broadcast_cancel", from_user=admin_user)
        await A.cb_bc_cancel(q, state)
        assert state.state is None
        assert state.data == {}
        assert any("отменена" in e.text.lower() for e in q.message.edits)

    async def test_broadcast_confirm_sends_to_active_users(self, admin_user) -> None:
        state = FSMData(state="BroadcastStates:preview", data={"bc_text": "Hello", "bc_entities": False})
        db = await make_memory_db()
        try:
            await db.execute(
                "INSERT INTO users (telegram_id, is_banned) VALUES (10, 0), (20, 0), (30, 1)",
            )
            await db.commit()

            bot = FakeBot()
            q = FakeCallbackQuery(data="broadcast_confirm", from_user=admin_user, bot=bot)
            await A.cb_bc_confirm(q, state, db)

            delivered_ids = sorted(m.chat_id for m in bot.sent)
            assert delivered_ids == [10, 20]
            assert state.state is None
            assert any("Рассылка завершена" in e.text for e in q.message.edits)
        finally:
            await db.close()

    async def test_broadcast_confirm_non_admin_exits(self, regular_user) -> None:
        state = FSMData(state="BroadcastStates:preview", data={"bc_text": "x"})
        db = await make_memory_db()
        try:
            q = FakeCallbackQuery(data="broadcast_confirm", from_user=regular_user)
            await A.cb_bc_confirm(q, state, db)
            assert q.message.edits == []
        finally:
            await db.close()

    async def test_admin_close_clears_state(self, admin_user) -> None:
        state = FSMData(state="BroadcastStates:preview", data={"x": 1})
        q = FakeCallbackQuery(data="admin_close", from_user=admin_user)
        await A.cb_admin_close(q, state)
        assert state.state is None
        assert state.data == {}
        assert any("Закрыто" in e.text for e in q.message.edits)

    async def test_bc_capture_non_admin_clears_state(self, regular_user) -> None:
        state = FSMData(state="BroadcastStates:waiting_text")
        msg = FakeMessage(text="evil", from_user=regular_user)
        await A.bc_capture(msg, state)
        assert state.state is None
