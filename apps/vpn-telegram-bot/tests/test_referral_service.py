"""Unit tests for the referral service DB helpers."""
from __future__ import annotations

from tests._stubs import make_memory_db
from vpn_bot.services.referral_service import (
    finalize_referral_after_payment,
    get_referrer_for_payment_bonus,
    record_referral,
    referral_stats,
)


class TestRecordReferral:
    async def test_self_referral_ignored(self) -> None:
        db = await make_memory_db()
        try:
            await record_referral(db, 10, 10)
            cur = await db.execute("SELECT COUNT(*) FROM referrals")
            (count,) = await cur.fetchone()
            assert count == 0
        finally:
            await db.close()

    async def test_insert_or_ignore(self) -> None:
        db = await make_memory_db()
        try:
            await record_referral(db, 10, 20)
            await record_referral(db, 30, 20)  # UNIQUE on referred_id
            cur = await db.execute("SELECT referrer_id FROM referrals WHERE referred_id = 20")
            (ref,) = await cur.fetchone()
            assert ref == 10  # first write wins
        finally:
            await db.close()


class TestStats:
    async def test_counts(self) -> None:
        db = await make_memory_db()
        try:
            await db.execute(
                "INSERT INTO referrals (referrer_id, referred_id, referred_paid) VALUES "
                "(1, 10, 0), (1, 11, 1), (1, 12, 1), (2, 20, 0)"
            )
            await db.commit()
            assert await referral_stats(db, 1) == (3, 2)
            assert await referral_stats(db, 2) == (1, 0)
            assert await referral_stats(db, 99) == (0, 0)
        finally:
            await db.close()


class TestBonusFlow:
    async def test_get_and_finalize(self) -> None:
        db = await make_memory_db()
        try:
            await record_referral(db, 100, 200)
            assert await get_referrer_for_payment_bonus(db, 200) == 100
            await finalize_referral_after_payment(db, 200)
            # After finalize, bonus_applied=1 → returns None to avoid double bonus.
            assert await get_referrer_for_payment_bonus(db, 200) is None
        finally:
            await db.close()

    async def test_no_referrer(self) -> None:
        db = await make_memory_db()
        try:
            assert await get_referrer_for_payment_bonus(db, 9999) is None
        finally:
            await db.close()
