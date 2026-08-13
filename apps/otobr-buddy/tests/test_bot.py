"""Tests for Telegram command handlers in bot.py."""

from unittest.mock import AsyncMock, MagicMock
from zoneinfo import ZoneInfo

from otobr_buddy import bot, database
from otobr_buddy.config import Config


def make_user(user_id: str = "1", username: str | None = "alice", first_name: str = "Alice"):
    user = MagicMock()
    user.id = int(user_id)
    user.username = username
    user.first_name = first_name
    return user


def make_update(
    user_id: str = "1",
    username: str | None = "alice",
    first_name: str = "Alice",
    chat_type: str = "private",
    chat_id: str | None = None,
):
    update = MagicMock()
    update.effective_user = make_user(user_id, username, first_name)
    update.effective_chat = MagicMock()
    update.effective_chat.type = chat_type
    update.effective_chat.id = int(chat_id) if chat_id is not None else int(user_id)
    update.message = MagicMock()
    update.message.reply_text = AsyncMock()
    return update


def make_context(args: list[str] | None = None, admin_chat_ids: list[str] | None = None):
    context = MagicMock()
    context.args = args or []
    context.bot_data = {
        "timezone": ZoneInfo("UTC"),
        "config": Config(admin_chat_ids=admin_chat_ids or []),
    }
    context.bot = MagicMock()
    context.bot.send_message = AsyncMock()
    context.job_queue = MagicMock()
    context.job_queue.get_jobs_by_name.return_value = []
    return context


def reply_text(update) -> str:
    return update.message.reply_text.call_args.args[0]


async def test_cmd_start_sends_help():
    update = make_update()
    await bot.cmd_start(update, make_context())
    assert "otobr-buddy" in reply_text(update)


# ---------------------------------------------------------------------------
# /pair (group-based auto-pairing)
# ---------------------------------------------------------------------------


async def test_cmd_pair_rejects_dm():
    update = make_update(chat_type="private")
    await bot.cmd_pair(update, make_context())
    assert "group chat" in reply_text(update).lower()


async def test_cmd_pair_first_claim_waits():
    update = make_update(user_id="1", chat_type="group", chat_id="-100123")
    await bot.cmd_pair(update, make_context())

    assert "waiting" in reply_text(update).lower()
    pending = database.get_pending_pair("-100123")
    assert pending is not None
    assert pending["claimant_id"] == "1"


async def test_cmd_pair_second_distinct_user_forms_partnership():
    first = make_update(user_id="1", username="alice", chat_type="group", chat_id="-100123")
    await bot.cmd_pair(first, make_context())

    second = make_update(user_id="2", username="bob", chat_type="group", chat_id="-100123")
    await bot.cmd_pair(second, make_context())

    assert "now reading with" in reply_text(second)
    assert database.get_pending_pair("-100123") is None

    partnership = database.get_active_partnership_for_group("-100123")
    assert partnership is not None
    assert {partnership["user_a_id"], partnership["user_b_id"]} == {"1", "2"}


async def test_cmd_pair_same_user_twice_still_waiting():
    update = make_update(user_id="1", chat_type="group", chat_id="-100123")
    await bot.cmd_pair(update, make_context())
    update.message.reply_text.reset_mock()

    await bot.cmd_pair(update, make_context())

    assert "still waiting" in reply_text(update).lower()
    assert database.get_active_partnership_for_group("-100123") is None


async def test_cmd_pair_rejected_when_group_already_paired():
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    database.create_partnership("1", "2", group_chat_id="-100123")

    third = make_update(user_id="3", username="carol", chat_type="group", chat_id="-100123")
    await bot.cmd_pair(third, make_context())

    assert "already linked" in reply_text(third).lower()


# ---------------------------------------------------------------------------
# /invite, /join (DM fallback)
# ---------------------------------------------------------------------------


async def test_invite_then_join_forms_partnership():
    inviter = make_update(user_id="1", username="alice")
    await bot.cmd_invite(inviter, make_context())
    code = reply_text(inviter).split("<code>")[1].split("</code>")[0]

    joiner = make_update(user_id="2", username="bob")
    await bot.cmd_join(joiner, make_context(args=[code]))

    assert "now reading with" in reply_text(joiner)
    partnerships = database.get_active_partnerships_for_user("2")
    assert len(partnerships) == 1
    assert partnerships[0]["user_a_id"] == "1"


async def test_join_rejects_own_invite():
    update = make_update(user_id="1")
    await bot.cmd_invite(update, make_context())
    code = reply_text(update).split("<code>")[1].split("</code>")[0]
    update.message.reply_text.reset_mock()

    await bot.cmd_join(update, make_context(args=[code]))

    assert "can't redeem your own" in reply_text(update).lower()


async def test_join_rejects_unknown_code():
    update = make_update(user_id="2")
    await bot.cmd_join(update, make_context(args=["NOPE99"]))
    assert "not found" in reply_text(update).lower()


async def test_join_requires_code_argument():
    update = make_update(user_id="2")
    await bot.cmd_join(update, make_context(args=[]))
    assert "usage" in reply_text(update).lower()


async def test_invite_rejects_group():
    update = make_update(user_id="1", chat_type="group", chat_id="-100123")
    await bot.cmd_invite(update, make_context())
    assert "privately" in reply_text(update).lower()


async def test_join_rejects_group():
    update = make_update(user_id="2", chat_type="group", chat_id="-100123")
    await bot.cmd_join(update, make_context(args=["ABCDEF"]))
    assert "privately" in reply_text(update).lower()


# ---------------------------------------------------------------------------
# /partners, /history
# ---------------------------------------------------------------------------


async def test_partners_rejects_group():
    update = make_update(user_id="1", chat_type="group", chat_id="-100123")
    await bot.cmd_partners(update, make_context())
    assert "privately" in reply_text(update).lower()


async def test_partners_lists_in_private():
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    database.create_partnership("1", "2")

    update = make_update(user_id="1")
    await bot.cmd_partners(update, make_context())

    assert "bob" in reply_text(update).lower()


async def test_history_rejects_group():
    update = make_update(user_id="1", chat_type="group", chat_id="-100123")
    await bot.cmd_history(update, make_context())
    assert "privately" in reply_text(update).lower()


# ---------------------------------------------------------------------------
# /log
# ---------------------------------------------------------------------------


async def test_log_without_partnership_prompts_pairing():
    update = make_update(user_id="1")
    await bot.cmd_log(update, make_context(args=["Romans", "8:1-17"]))
    assert "/pair" in reply_text(update)


async def test_log_records_session():
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    database.create_partnership("1", "2")

    update = make_update(user_id="1")
    await bot.cmd_log(update, make_context(args=["Romans", "8:1-17"]))

    assert "logged" in reply_text(update).lower()
    partnership = database.get_active_partnerships_for_user("1")[0]
    sessions = database.get_sessions_for_partnership(partnership["id"])
    assert len(sessions) == 1
    assert sessions[0]["text_covered"] == "Romans 8:1-17"


async def test_log_reschedules_reminder_in_interval_mode():
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    partnership_id = database.create_partnership("1", "2")
    database.set_frequency_interval(partnership_id, 3)

    update = make_update(user_id="1")
    context = make_context(args=["Romans 1"])
    await bot.cmd_log(update, context)

    context.job_queue.run_once.assert_called_once()


# ---------------------------------------------------------------------------
# /setfrequency
# ---------------------------------------------------------------------------


async def test_setfrequency_interval_valid():
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    partnership_id = database.create_partnership("1", "2")

    update = make_update(user_id="1")
    await bot.cmd_setfrequency(update, make_context(args=["interval", "5"]))

    assert "every 5 day" in reply_text(update)
    partnership = database.get_partnership(partnership_id)
    assert partnership is not None
    assert partnership["frequency_interval_days"] == 5


async def test_setfrequency_interval_out_of_range_rejected():
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    database.create_partnership("1", "2")

    update = make_update(user_id="1")
    await bot.cmd_setfrequency(update, make_context(args=["interval", "999"]))

    assert "between 1 and 365" in reply_text(update)


async def test_setfrequency_weekly_valid():
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    partnership_id = database.create_partnership("1", "2")

    update = make_update(user_id="1")
    await bot.cmd_setfrequency(update, make_context(args=["weekly", "wed", "19:00"]))

    partnership = database.get_partnership(partnership_id)
    assert partnership is not None
    assert partnership["frequency_day_of_week"] == 2
    assert partnership["frequency_time"] == "19:00"


async def test_setfrequency_weekly_bad_time_rejected():
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    database.create_partnership("1", "2")

    update = make_update(user_id="1")
    await bot.cmd_setfrequency(update, make_context(args=["weekly", "wed", "25:00"]))

    assert "24h" in reply_text(update)


# ---------------------------------------------------------------------------
# /link
# ---------------------------------------------------------------------------


async def test_link_rejects_outside_group():
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    database.create_partnership("1", "2")

    update = make_update(user_id="1", chat_type="private")
    await bot.cmd_link(update, make_context())

    assert "group chat" in reply_text(update).lower()


async def test_link_guards_against_double_linking_group():
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    database.upsert_user("3", "carol", "Carol")
    database.create_partnership("1", "2", group_chat_id="-100123")
    database.create_partnership("1", "3")

    update = make_update(user_id="1", chat_type="group", chat_id="-100123")
    await bot.cmd_link(update, make_context(args=["#2"]))

    assert "already linked" in reply_text(update).lower()


# ---------------------------------------------------------------------------
# /end (via inline keyboard callback)
# ---------------------------------------------------------------------------


async def test_end_callback_ends_partnership():
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    partnership_id = database.create_partnership("1", "2")

    update = MagicMock()
    update.callback_query = MagicMock()
    update.callback_query.data = f"end_confirm:{partnership_id}"
    update.callback_query.from_user = make_user(user_id="1")
    update.callback_query.answer = AsyncMock()
    update.callback_query.edit_message_text = AsyncMock()

    await bot.on_end_callback(update, make_context())

    partnership = database.get_partnership(partnership_id)
    assert partnership is not None
    assert partnership["status"] == database.STATUS_ENDED
    update.callback_query.edit_message_text.assert_called_once()
    assert "ended" in update.callback_query.edit_message_text.call_args.args[0].lower()


async def test_end_callback_rejects_non_member():
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    partnership_id = database.create_partnership("1", "2")

    update = MagicMock()
    update.callback_query = MagicMock()
    update.callback_query.data = f"end_confirm:{partnership_id}"
    update.callback_query.from_user = make_user(user_id="999")
    update.callback_query.answer = AsyncMock()
    update.callback_query.edit_message_text = AsyncMock()

    await bot.on_end_callback(update, make_context())

    partnership = database.get_partnership(partnership_id)
    assert partnership is not None
    assert partnership["status"] == database.STATUS_ACTIVE


# ---------------------------------------------------------------------------
# /debug (admin only)
# ---------------------------------------------------------------------------


async def test_debug_rejects_non_admin():
    update = make_update(user_id="1")
    await bot.cmd_debug(update, make_context(admin_chat_ids=["999"]))
    assert "restricted to administrators" in reply_text(update).lower()


async def test_debug_unrestricted_when_no_admins_configured():
    update = make_update(user_id="1")
    await bot.cmd_debug(update, make_context())
    assert "debug info" in reply_text(update).lower()


async def test_debug_shows_stats_for_admin():
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    database.create_partnership("1", "2")

    update = make_update(user_id="1")
    await bot.cmd_debug(update, make_context(admin_chat_ids=["1"]))

    text = reply_text(update)
    assert "total users: 2" in text.lower()
    assert "active partnerships: 1" in text.lower()
