"""Handler-level tests for purchase FSM flow.

We invoke aiogram handler functions directly with hand-rolled stubs to cover
the full purchase funnel: plans -> pay methods -> SBP page -> confirm_paid ->
admin_confirm / admin_reject. Traffic goes through `VPNBackend` (stubbed).
"""
from __future__ import annotations

import json
from typing import Any

import pytest

from tests._stubs import FakeBot, FakeCallbackQuery, FakeMessage, FakeUser, make_memory_db
from vpn_bot.handlers import purchase as P


class PurchaseBackend:
    """Minimal `VPNBackend` stub that records all calls."""

    def __init__(self) -> None:
        self.issue_status_resp: tuple[int, dict[str, Any]] = (404, {"message": "not found"})
        self.issue_link_resp: tuple[int, dict[str, Any]] = (200, {"subscription": {"id": "uuid-new"}})
        self.lifecycle_renew_resp: tuple[int, dict[str, Any]] = (200, {"ok": True})
        self.renew_calls: list[tuple[str, int]] = []
        self.issue_link_calls: list[tuple] = []

    async def issue_status(self, user_id: str) -> tuple[int, dict[str, Any]]:
        return self.issue_status_resp

    async def issue_link(self, user_id: str, name: str, source: str, profile_ids, idem):
        self.issue_link_calls.append((user_id, name, source, profile_ids, idem))
        return self.issue_link_resp

    async def lifecycle_renew(self, user_id: str, days: int):
        self.renew_calls.append((user_id, days))
        return self.lifecycle_renew_resp

    async def get_subscription(self, sid: str):
        return 200, {}

    async def get_delivery_links(self, pid: str):
        return 200, {}

    async def get_health(self):
        return 200, {}


@pytest.fixture
def user() -> FakeUser:
    return FakeUser(id=1000, username="buyer")


@pytest.fixture
def bot() -> FakeBot:
    return FakeBot()


class TestApplyPaidMonths:
    async def test_new_user_one_month_creates_subscription_only(self) -> None:
        api = PurchaseBackend()
        api.issue_status_resp = (404, {})

        ok, err = await P._apply_paid_months(api, telegram_user_id=42, months=1)

        assert ok is True
        assert err == ""
        assert len(api.issue_link_calls) == 1
        assert api.renew_calls == []

    async def test_new_user_multi_month_also_extends(self) -> None:
        api = PurchaseBackend()
        api.issue_status_resp = (404, {})

        ok, _ = await P._apply_paid_months(api, telegram_user_id=42, months=3)

        assert ok is True
        assert len(api.issue_link_calls) == 1
        assert api.renew_calls == [("tg_42", 60)]  # 3*30 - 30

    async def test_existing_user_only_extends(self) -> None:
        api = PurchaseBackend()
        api.issue_status_resp = (200, {"subscriptionId": "old-uuid"})

        ok, _ = await P._apply_paid_months(api, telegram_user_id=42, months=2)

        assert ok is True
        assert api.issue_link_calls == []
        assert api.renew_calls == [("tg_42", 60)]

    async def test_renew_failure_surfaced(self) -> None:
        api = PurchaseBackend()
        api.issue_status_resp = (200, {"subscriptionId": "old-uuid"})
        api.lifecycle_renew_resp = (500, {"error": "boom"})

        ok, err = await P._apply_paid_months(api, telegram_user_id=42, months=1)
        assert ok is False
        assert "boom" in err

    async def test_issue_link_failure_surfaced_new_user(self) -> None:
        api = PurchaseBackend()
        api.issue_status_resp = (404, {})
        api.issue_link_resp = (502, {"message": "bad gateway"})

        ok, err = await P._apply_paid_months(api, telegram_user_id=42, months=1)
        assert ok is False
        assert "bad gateway" in err


class TestOpenPurchaseMessage:
    async def test_open_purchase_replies_with_plans(self, user, bot) -> None:
        msg = FakeMessage(text="💎 Оплатить подписку", from_user=user, bot=bot)
        await P.open_purchase(msg)
        assert len(msg.answers) == 1
        assert "Выбери тариф" in msg.answers[0].text
        assert msg.answers[0].reply_markup is not None

    async def test_cb_open_purchase_plans_sends_when_no_message(self, user) -> None:
        bot = FakeBot()
        query = FakeCallbackQuery(data="open_purchase_plans", from_user=user, bot=bot, message=None)
        query.message = None  # explicit: no message
        await P.cb_open_purchase_plans(query)
        assert any("Выбери тариф" in m.text for m in bot.sent)


class TestPlanSelection:
    async def test_plan_callback_edits_with_pay_methods(self, user) -> None:
        q = FakeCallbackQuery(data="plan_6", from_user=user)
        await P.cb_plan(q)
        assert q.answered
        assert any("6 мес." in e.text for e in q.message.edits)

    async def test_plan_callback_invalid_months_ignored(self, user) -> None:
        q = FakeCallbackQuery(data="plan_abc", from_user=user)
        await P.cb_plan(q)
        assert q.message.edits == []

    async def test_plan_callback_unknown_plan_ignored(self, user) -> None:
        q = FakeCallbackQuery(data="plan_99", from_user=user)
        await P.cb_plan(q)
        assert q.message.edits == []


class TestPayCard:
    async def test_no_provider_token_shows_hint(self, monkeypatch, user) -> None:
        from vpn_bot.config import settings
        monkeypatch.setattr(settings, "payment_provider_token", "", raising=False)

        q = FakeCallbackQuery(data="pay_card_1", from_user=user)
        await P.cb_pay_card(q)
        assert any("Telegram Payments не настроены" in e.text for e in q.message.edits)

    async def test_with_provider_token_sends_invoice(self, monkeypatch, user) -> None:
        from vpn_bot.config import settings
        monkeypatch.setattr(settings, "payment_provider_token", "tok", raising=False)
        monkeypatch.setattr(settings, "payment_currency", "RUB", raising=False)

        bot = FakeBot()
        q = FakeCallbackQuery(data="pay_card_2", from_user=user, bot=bot)
        await P.cb_pay_card(q)
        assert len(bot.invoices) == 1
        assert bot.invoices[0]["chat_id"] == user.id
        assert bot.invoices[0]["provider_token"] == "tok"


class TestPreCheckout:
    async def test_pre_checkout_accepts(self) -> None:
        answered: list[bool] = []

        class _PQ:
            async def answer(self, *, ok: bool) -> None:
                answered.append(ok)

        await P.pre_checkout(_PQ())
        assert answered == [True]


class TestSbpFlow:
    async def test_pay_sbp_opens_sbp_page(self, user) -> None:
        api = PurchaseBackend()
        q = FakeCallbackQuery(data="pay_sbp_1", from_user=user)
        await P.cb_pay_sbp(q, api)
        assert any("Оплата по СБП" in e.text for e in q.message.edits)

    async def test_pay_manual_legacy_still_works(self, user) -> None:
        api = PurchaseBackend()
        q = FakeCallbackQuery(data="pay_manual_2", from_user=user)
        await P.cb_pay_manual_legacy(q, api)
        assert any("Оплата по СБП" in e.text for e in q.message.edits)

    async def test_sbp_stub_answers_with_alert(self, user) -> None:
        q = FakeCallbackQuery(data="sbp_pay_stub_1", from_user=user)
        await P.cb_sbp_pay_stub(q)
        assert q.answered
        _, show_alert = q.answered[0]
        assert show_alert is True


class TestConfirmPaidStub:
    async def test_stub_ok_path(self, monkeypatch, user) -> None:
        from vpn_bot.config import settings
        monkeypatch.setattr(settings, "payment_stub_enabled", True, raising=False)
        monkeypatch.setattr(settings, "payment_stub_result", "ok", raising=False)
        api = PurchaseBackend()
        db = await make_memory_db()
        try:
            q = FakeCallbackQuery(data="confirm_paid_1", from_user=user)
            await P.cb_confirm_paid(q, db, api)
            assert any("зачислен" in e.text for e in q.message.edits)
        finally:
            await db.close()

    async def test_stub_fail_path(self, monkeypatch, user) -> None:
        from vpn_bot.config import settings
        monkeypatch.setattr(settings, "payment_stub_enabled", True, raising=False)
        monkeypatch.setattr(settings, "payment_stub_result", "fail", raising=False)
        api = PurchaseBackend()
        db = await make_memory_db()
        try:
            q = FakeCallbackQuery(data="confirm_paid_1", from_user=user)
            await P.cb_confirm_paid(q, db, api)
            assert any("не прошёл" in e.text for e in q.message.edits)
        finally:
            await db.close()


class TestConfirmPaidRealFlowNotifiesAdmins:
    async def test_creates_pending_payment_and_pings_admins(self, monkeypatch, user) -> None:
        from vpn_bot.config import settings
        monkeypatch.setattr(settings, "payment_stub_enabled", False, raising=False)
        monkeypatch.setattr(settings, "admin_ids_csv", "777,888", raising=False)

        api = PurchaseBackend()
        db = await make_memory_db()
        try:
            q = FakeCallbackQuery(data="confirm_paid_2", from_user=user)
            await P.cb_confirm_paid(q, db, api)

            cur = await db.execute("SELECT months, amount, status, payment_method FROM payments")
            rows = await cur.fetchall()
            assert len(rows) == 1
            months, amount, status, method = rows[0]
            assert months == 2
            assert amount == 17000  # 170 ₽ -> kopecks
            assert status == "pending"
            assert method == "manual"

            admin_msgs = [m for m in q.bot.sent if m.chat_id in (777, 888)]
            assert {m.chat_id for m in admin_msgs} == {777, 888}
            assert any("payment_id" in m.text for m in admin_msgs)

            assert any("Заявка отправлена" in e.text for e in q.message.edits)
        finally:
            await db.close()


class TestAdminConfirm:
    async def test_non_admin_is_rejected(self, monkeypatch, user) -> None:
        from vpn_bot.config import settings
        monkeypatch.setattr(settings, "admin_ids_csv", "999", raising=False)

        api = PurchaseBackend()
        db = await make_memory_db()
        try:
            q = FakeCallbackQuery(data="admin_confirm_1", from_user=user)  # user.id=1000
            await P.cb_admin_confirm(q, api, db)
            assert q.answered and q.answered[0] == ("Нет прав", True)
        finally:
            await db.close()

    async def test_confirm_pending_payment(self, monkeypatch, user) -> None:
        from vpn_bot.config import settings
        monkeypatch.setattr(settings, "admin_ids_csv", "1000", raising=False)
        monkeypatch.setattr(settings, "referral_bonus_days", 15, raising=False)

        api = PurchaseBackend()
        api.issue_status_resp = (200, {"subscriptionId": "x"})  # existing user
        db = await make_memory_db()
        try:
            await db.execute(
                "INSERT INTO payments (user_telegram_id, months, amount, currency, status, payment_method)"
                " VALUES (?, ?, ?, 'RUB', 'pending', 'manual')",
                (2001, 1, 9000),
            )
            await db.commit()
            cur = await db.execute("SELECT id FROM payments ORDER BY id DESC LIMIT 1")
            pid = (await cur.fetchone())[0]

            q = FakeCallbackQuery(data=f"admin_confirm_{pid}", from_user=user)
            await P.cb_admin_confirm(q, api, db)

            assert api.renew_calls == [("tg_2001", 30)]
            cur = await db.execute("SELECT status, confirmed_by FROM payments WHERE id=?", (pid,))
            status, confirmed_by = await cur.fetchone()
            assert status == "confirmed"
            assert confirmed_by == 1000
            assert any("подтверждена" in e.text for e in q.message.edits)
        finally:
            await db.close()

    async def test_confirm_unknown_payment_shows_message(self, monkeypatch, user) -> None:
        from vpn_bot.config import settings
        monkeypatch.setattr(settings, "admin_ids_csv", "1000", raising=False)

        api = PurchaseBackend()
        db = await make_memory_db()
        try:
            q = FakeCallbackQuery(data="admin_confirm_999", from_user=user)
            await P.cb_admin_confirm(q, api, db)
            assert any("не найдена" in e.text for e in q.message.edits)
        finally:
            await db.close()


class TestAdminReject:
    async def test_reject_cancels_payment(self, monkeypatch, user) -> None:
        from vpn_bot.config import settings
        monkeypatch.setattr(settings, "admin_ids_csv", "1000", raising=False)

        db = await make_memory_db()
        try:
            await db.execute(
                "INSERT INTO payments (user_telegram_id, months, amount, status, payment_method)"
                " VALUES (5, 1, 9000, 'pending', 'manual')",
            )
            await db.commit()
            cur = await db.execute("SELECT id FROM payments")
            pid = (await cur.fetchone())[0]

            q = FakeCallbackQuery(data=f"admin_reject_{pid}", from_user=user)
            await P.cb_admin_reject(q, db)
            cur = await db.execute("SELECT status FROM payments WHERE id=?", (pid,))
            (status,) = await cur.fetchone()
            assert status == "cancelled"
            assert any("отклонена" in e.text for e in q.message.edits)
        finally:
            await db.close()


class TestSuccessfulPayment:
    async def test_successful_payment_extends_and_notifies(self, monkeypatch, user) -> None:
        from vpn_bot.config import settings
        monkeypatch.setattr(settings, "referral_bonus_days", 15, raising=False)

        api = PurchaseBackend()
        api.issue_status_resp = (200, {"subscriptionId": "x"})  # existing user
        db = await make_memory_db()

        class _SP:
            invoice_payload = json.dumps({"m": 2, "u": user.id, "t": 0})

        try:
            msg = FakeMessage(text="", from_user=user, successful_payment=_SP())
            await P.on_successful_payment(msg, api, db)
            assert api.renew_calls == [("tg_1000", 60)]
            assert any("Оплата подтверждена" in a.text for a in msg.answers)
        finally:
            await db.close()


class TestBackNavigation:
    async def test_back_plans_shows_plans(self, user) -> None:
        q = FakeCallbackQuery(data="back_plans", from_user=user)
        await P.cb_back_plans(q)
        assert any("Выбери тариф" in e.text for e in q.message.edits)

    async def test_back_pay_returns_to_pay_methods(self, user) -> None:
        q = FakeCallbackQuery(data="back_pay_3", from_user=user)
        await P.cb_back_pay(q)
        assert any("3 мес." in e.text for e in q.message.edits)

    async def test_back_main_shows_hint(self, user) -> None:
        q = FakeCallbackQuery(data="back_main", from_user=user)
        await P.cb_back_main(q)
        assert any("меню" in e.text.lower() for e in q.message.edits)
