"""Shared aiogram test stubs (Message/CallbackQuery/Bot) and an in-memory DB.

These are deliberately minimal: just enough surface area to drive the router
handlers directly in unit tests (without spinning up a real Bot/Dispatcher).
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

import aiosqlite


@dataclass
class RecMsg:
    chat_id: int
    text: str
    reply_markup: Any = None
    parse_mode: str | None = None


class FakeBot:
    def __init__(self) -> None:
        self.sent: list[RecMsg] = []
        self.invoices: list[dict[str, Any]] = []

    async def send_message(
        self,
        chat_id: int,
        text: str,
        *,
        reply_markup: Any = None,
        parse_mode: str | None = None,
    ) -> None:
        self.sent.append(RecMsg(chat_id=chat_id, text=text, reply_markup=reply_markup, parse_mode=parse_mode))

    async def send_invoice(self, **kwargs: Any) -> None:
        self.invoices.append(kwargs)


@dataclass
class FakeUser:
    id: int
    username: str | None = None
    full_name: str = "Test User"


@dataclass
class FakeChat:
    id: int = 1


class FakeMessage:
    def __init__(
        self,
        *,
        text: str = "",
        from_user: FakeUser | None = None,
        bot: FakeBot | None = None,
        html_text: str | None = None,
        entities: list[Any] | None = None,
        successful_payment: Any = None,
    ) -> None:
        self.text = text
        self.from_user = from_user
        self.bot = bot or FakeBot()
        self.chat = FakeChat(id=from_user.id if from_user else 1)
        self.html_text = html_text
        self.entities = entities or []
        self.successful_payment = successful_payment
        self.answers: list[RecMsg] = []
        self.edits: list[RecMsg] = []

    async def answer(self, text: str, *, reply_markup: Any = None, parse_mode: str | None = None) -> None:
        self.answers.append(RecMsg(chat_id=self.chat.id, text=text, reply_markup=reply_markup, parse_mode=parse_mode))

    async def edit_text(self, text: str, *, reply_markup: Any = None, parse_mode: str | None = None) -> None:
        self.edits.append(RecMsg(chat_id=self.chat.id, text=text, reply_markup=reply_markup, parse_mode=parse_mode))


class FakeCallbackQuery:
    def __init__(
        self,
        *,
        data: str,
        from_user: FakeUser,
        bot: FakeBot | None = None,
        message: FakeMessage | None = None,
    ) -> None:
        self.data = data
        self.from_user = from_user
        self.bot = bot or (message.bot if message else FakeBot())
        self.message = message or FakeMessage(from_user=from_user, bot=self.bot)
        self.answered: list[tuple[str | None, bool]] = []

    async def answer(self, text: str | None = None, *, show_alert: bool = False) -> None:
        self.answered.append((text, show_alert))


@dataclass
class FSMData:
    """Tiny in-memory FSMContext replacement."""
    state: str | None = None
    data: dict[str, Any] = field(default_factory=dict)

    async def set_state(self, state: Any) -> None:
        self.state = getattr(state, "state", str(state))

    async def get_state(self) -> str | None:
        return self.state

    async def update_data(self, **kw: Any) -> None:
        self.data.update(kw)

    async def get_data(self) -> dict[str, Any]:
        return dict(self.data)

    async def clear(self) -> None:
        self.state = None
        self.data.clear()


async def make_memory_db() -> aiosqlite.Connection:
    """Create an in-memory SQLite DB with the bot schema applied."""
    from vpn_bot.database.models import SCHEMA

    db = await aiosqlite.connect(":memory:")
    await db.executescript(SCHEMA)
    await db.commit()
    return db
