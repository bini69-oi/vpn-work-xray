from __future__ import annotations

import logging

import aiosqlite

from vpn_bot.config import settings
from vpn_bot.database.migrations import ensure_schema_migrations
from vpn_bot.database.models import SCHEMA

log = logging.getLogger(__name__)


async def init_db() -> None:
    path = settings.database_file()
    path.parent.mkdir(parents=True, exist_ok=True)
    async with aiosqlite.connect(path) as db:
        await db.executescript(SCHEMA)
        await ensure_schema_migrations(db)
        await db.commit()
    log.info("database ready at %s", path)


async def db_connect() -> aiosqlite.Connection:
    path = settings.database_file()
    path.parent.mkdir(parents=True, exist_ok=True)
    db = await aiosqlite.connect(path)
    db.row_factory = aiosqlite.Row
    return db
