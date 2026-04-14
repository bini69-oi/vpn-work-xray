from __future__ import annotations

import aiosqlite


async def record_referral(db: aiosqlite.Connection, referrer_tg_id: int, referred_tg_id: int) -> None:
    if referrer_tg_id == referred_tg_id:
        return
    await db.execute(
        "INSERT OR IGNORE INTO referrals (referrer_id, referred_id) VALUES (?, ?)",
        (referrer_tg_id, referred_tg_id),
    )
    await db.commit()


async def referral_stats(db: aiosqlite.Connection, referrer_tg_id: int) -> tuple[int, int]:
    cur = await db.execute(
        "SELECT COUNT(*) FROM referrals WHERE referrer_id = ?",
        (referrer_tg_id,),
    )
    row = await cur.fetchone()
    invited = int(row[0]) if row else 0
    cur2 = await db.execute(
        "SELECT COUNT(*) FROM referrals WHERE referrer_id = ? AND referred_paid = 1",
        (referrer_tg_id,),
    )
    row2 = await cur2.fetchone()
    paid = int(row2[0]) if row2 else 0
    return invited, paid


async def get_referrer_for_payment_bonus(db: aiosqlite.Connection, buyer_tg_id: int) -> int | None:
    cur = await db.execute(
        "SELECT referrer_id, bonus_applied FROM referrals WHERE referred_id = ?",
        (buyer_tg_id,),
    )
    row = await cur.fetchone()
    if not row:
        return None
    if int(row[1]):
        return None
    return int(row[0])


async def finalize_referral_after_payment(db: aiosqlite.Connection, buyer_tg_id: int) -> None:
    await db.execute(
        "UPDATE referrals SET referred_paid = 1, bonus_applied = 1 WHERE referred_id = ?",
        (buyer_tg_id,),
    )
    await db.commit()
