from __future__ import annotations

import logging
import time
from typing import Any, Awaitable, Callable

from aiogram import BaseMiddleware

log = logging.getLogger(__name__)

_MIN_INTERVAL = 0.5
_last: dict[int, float] = {}


class ThrottlingMiddleware(BaseMiddleware):
    async def __call__(
        self,
        handler: Callable[[Any, dict[str, Any]], Awaitable[Any]],
        event: Any,
        data: dict[str, Any],
    ) -> Any:
        uid = getattr(getattr(event, "from_user", None), "id", None)
        if uid is None:
            return await handler(event, data)
        now = time.monotonic()
        prev = _last.get(uid, 0.0)
        if now - prev < _MIN_INTERVAL:
            log.debug("throttle skip user=%s", uid)
            return None
        _last[uid] = now
        return await handler(event, data)
