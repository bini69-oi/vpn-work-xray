"""Handler-level tests: start, subscription (My VPN + HApp), support, referral.

Covers the remaining entrypoints that were previously untested so we can hit
the coverage gate with a realistic safety net.
"""
from __future__ import annotations

from typing import Any

import pytest

from tests._stubs import FakeBot, FakeCallbackQuery, FakeMessage, FakeUser, make_memory_db
from vpn_bot.handlers import referral as R
from vpn_bot.handlers import start as S
from vpn_bot.handlers import subscription as SUB
from vpn_bot.handlers import support as SUP


class SubBackend:
    def __init__(
        self,
        status: tuple[int, dict[str, Any]] = (404, {}),
        sub: tuple[int, dict[str, Any]] = (200, {}),
        links: tuple[int, dict[str, Any]] = (200, {"links": {}}),
    ) -> None:
        self.status = status
        self.sub = sub
        self.links = links

    async def issue_status(self, user_id: str):
        return self.status

    async def issue_link(self, *a, **kw):
        return 200, {}

    async def lifecycle_renew(self, user_id: str, days: int):
        return 200, {}

    async def get_subscription(self, sid: str):
        return self.sub

    async def get_delivery_links(self, pid: str):
        return self.links

    async def get_health(self):
        return 200, {}


@pytest.fixture
def user() -> FakeUser:
    return FakeUser(id=42, username="u42")


# ---------------- START ----------------

class TestStart:
    async def test_plain_start_welcomes_and_shows_menu(self, user) -> None:
        db = await make_memory_db()
        try:
            msg = FakeMessage(text="/start", from_user=user)
            await S.cmd_start(msg, db, None)
            assert any("Добро пожаловать" in a.text for a in msg.answers)
            assert any("Используй кнопки" in a.text for a in msg.answers)
        finally:
            await db.close()

    async def test_referral_start_records_and_notifies(self, user) -> None:
        db = await make_memory_db()
        try:
            msg = FakeMessage(text="/start ref_777", from_user=user)
            await S.cmd_start(msg, db, None)

            cur = await db.execute("SELECT referrer_id, referred_id FROM referrals")
            rows = await cur.fetchall()
            assert rows == [(777, user.id)]
            assert any(m.chat_id == 777 for m in msg.bot.sent)
        finally:
            await db.close()

    async def test_about_vpn_short(self, user) -> None:
        msg = FakeMessage(text="📋 О VPN-32", from_user=user)
        await S.cmd_about_vpn(msg, None)
        assert any("VPN-32" in a.text for a in msg.answers)


# ---------------- MY VPN ----------------

class TestMyVpn:
    async def test_service_unavailable_when_api_is_none(self, user) -> None:
        msg = FakeMessage(text="🛡 Мой VPN", from_user=user)
        await SUB.my_vpn(msg, None)
        assert any("временно недоступен" in a.text for a in msg.answers)

    async def test_no_subscription_branch(self, user) -> None:
        # `fetch_subscription_bundle` returns (200, status, None) when issue_status
        # is 200 but there is no subscriptionId yet.
        api = SubBackend(status=(200, {}))
        msg = FakeMessage(text="🛡 Мой VPN", from_user=user)
        await SUB.my_vpn(msg, api)
        assert any("Подписка не найдена" in a.text for a in msg.answers)

    async def test_active_subscription_renders_profile(self, user) -> None:
        future = "2100-01-01T00:00:00Z"
        api = SubBackend(
            status=(200, {"subscriptionId": "abc"}),
            sub=(200, {"expiresAt": future, "usedTrafficBytes": 1024 * 1024, "trafficLimitBytes": 0}),
        )
        msg = FakeMessage(text="🛡 Мой VPN", from_user=user)
        await SUB.my_vpn(msg, api)
        assert any("Мой VPN" in a.text for a in msg.answers)
        assert any("безлимит" in a.text for a in msg.answers)


class TestSubHapp:
    async def test_https_import_link_is_wrapped_in_happ(self, user) -> None:
        api = SubBackend(links=(200, {"links": {"subscription": "https://panel.example/sub/xyz"}}))
        q = FakeCallbackQuery(data="sub_happ", from_user=user)
        await SUB.cb_sub_happ(q, api)
        assert q.message.edits
        markup = q.message.edits[0].reply_markup
        assert markup is not None
        btn_urls = [b.url for row in markup.inline_keyboard for b in row if b.url]
        assert any(u.startswith("happ://add/https://") for u in btn_urls)

    async def test_missing_link_alerts(self, user) -> None:
        api = SubBackend(links=(200, {"links": {}}))
        q = FakeCallbackQuery(data="sub_happ", from_user=user)
        await SUB.cb_sub_happ(q, api)
        assert any(show_alert for _, show_alert in q.answered)

    async def test_non_200_alerts(self, user) -> None:
        api = SubBackend(links=(502, {"links": {}}))
        q = FakeCallbackQuery(data="sub_happ", from_user=user)
        await SUB.cb_sub_happ(q, api)
        assert any(show_alert for _, show_alert in q.answered)


class TestSubBack:
    async def test_no_sub_shows_none_text(self, user) -> None:
        api = SubBackend(status=(404, {}))
        q = FakeCallbackQuery(data="sub_back_profile", from_user=user)
        await SUB.cb_sub_back(q, api)
        assert any("Подписка не найдена" in e.text for e in q.message.edits)

    async def test_back_renders_profile(self, user) -> None:
        api = SubBackend(
            status=(200, {"subscriptionId": "abc"}),
            sub=(200, {"expiresAt": "2100-01-01T00:00:00Z"}),
        )
        q = FakeCallbackQuery(data="sub_back_profile", from_user=user)
        await SUB.cb_sub_back(q, api)
        assert any("Мой VPN" in e.text for e in q.message.edits)


class TestRenewHint:
    async def test_renders_renew_text(self, user) -> None:
        q = FakeCallbackQuery(data="sub_renew_hint", from_user=user)
        await SUB.cb_renew_hint(q)
        assert any("Продление" in e.text for e in q.message.edits)


# ---------------- SUPPORT ----------------

class TestSupport:
    async def test_help_menu_renders_faq(self, user) -> None:
        msg = FakeMessage(text="💬 Помощь", from_user=user)
        await SUP.cmd_help_menu(msg)
        assert any("Помощь" in a.text for a in msg.answers)

    async def test_faq_callbacks_switch_bodies(self, user) -> None:
        for data, needle, fn in [
            ("faq_connect", "подключиться", SUP.cb_faq_connect),
            ("faq_vpn", "Не работает VPN", SUP.cb_faq_vpn),
            ("faq_pay", "оплате", SUP.cb_faq_pay),
            ("faq_menu", "Помощь", SUP.cb_faq_menu),
        ]:
            q = FakeCallbackQuery(data=data, from_user=user)
            await fn(q)
            assert any(needle in e.text for e in q.message.edits), data


# ---------------- REFERRAL ----------------

class _Dispatcher:
    def __init__(self, *, bot_username: str | None = None) -> None:
        self._kv = {"bot_username": bot_username}

    def __getitem__(self, key: str) -> Any:
        v = self._kv.get(key)
        if not v:
            raise KeyError(key)
        return v


class TestReferral:
    async def test_requires_bot_username(self, user) -> None:
        db = await make_memory_db()
        try:
            msg = FakeMessage(text="🎁 Друзьям", from_user=user)
            await R.cmd_referral(msg, db, _Dispatcher(bot_username=None))
            assert any("BOT_USERNAME" in a.text for a in msg.answers)
        finally:
            await db.close()

    async def test_referral_stats_message(self, user) -> None:
        db = await make_memory_db()
        try:
            await db.execute("INSERT INTO referrals (referrer_id, referred_id) VALUES (?, ?)", (user.id, 100))
            await db.commit()
            msg = FakeMessage(text="🎁 Друзьям", from_user=user)
            await R.cmd_referral(msg, db, _Dispatcher(bot_username="my_bot"))
            assert any("Реферальная программа" in a.text for a in msg.answers)
            assert any("my_bot" in a.text for a in msg.answers)
        finally:
            await db.close()

    async def test_forward_pack_sends_card(self, user) -> None:
        bot = FakeBot()
        q = FakeCallbackQuery(data="referral_fwd", from_user=user, bot=bot)
        await R.cb_referral_forward_pack(q, _Dispatcher(bot_username="my_bot"))
        assert any("Отправь другу" in m.text for m in bot.sent)

    async def test_forward_pack_no_username(self, user) -> None:
        bot = FakeBot()
        q = FakeCallbackQuery(data="referral_fwd", from_user=user, bot=bot)
        await R.cb_referral_forward_pack(q, _Dispatcher(bot_username=None))
        assert any("BOT_USERNAME" in m.text for m in bot.sent)
