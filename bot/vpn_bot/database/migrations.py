from __future__ import annotations

import aiosqlite

# Увеличь при изменении reply-клавиатуры — пользователям пришлётся обновление.
REPLY_KEYBOARD_VERSION = 3


async def ensure_schema_migrations(db: aiosqlite.Connection) -> None:
    cur = await db.execute("PRAGMA table_info(users)")
    cols = {str(row[1]) for row in await cur.fetchall()}
    if "reply_menu_version" not in cols:
        await db.execute(
            "ALTER TABLE users ADD COLUMN reply_menu_version INTEGER NOT NULL DEFAULT 0",
        )
