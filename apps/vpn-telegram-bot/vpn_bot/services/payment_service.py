from __future__ import annotations

import time

import aiosqlite


async def create_pending_payment(
    db: aiosqlite.Connection,
    user_telegram_id: int,
    months: int,
    amount_rub: int,
    method: str,
) -> int:
    cur = await db.execute(
        """
        INSERT INTO payments (user_telegram_id, months, amount, currency, status, payment_method)
        VALUES (?, ?, ?, 'RUB', 'pending', ?)
        """,
        (user_telegram_id, months, int(amount_rub * 100), method),
    )
    await db.commit()
    return int(cur.lastrowid)


async def confirm_payment(db: aiosqlite.Connection, payment_id: int, admin_tg_id: int) -> None:
    now = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    await db.execute(
        """
        UPDATE payments SET status = 'confirmed', confirmed_by = ?, confirmed_at = ?
        WHERE id = ? AND status = 'pending'
        """,
        (admin_tg_id, now, payment_id),
    )
    await db.commit()
