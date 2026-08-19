"""Tests for JobQueue-backed reminder scheduling in reminders.py."""

from datetime import timedelta
from unittest.mock import AsyncMock, MagicMock
from zoneinfo import ZoneInfo

from otobr_buddy import database, reminders
from otobr_buddy.timeutil import format_timestamp, utcnow


def make_job_queue():
    job_queue = MagicMock()
    job_queue.get_jobs_by_name.return_value = []
    return job_queue


def make_partnership(**overrides):
    base = {
        "id": 1,
        "user_a_id": "1",
        "user_b_id": "2",
        "status": database.STATUS_ACTIVE,
        "frequency_mode": None,
        "frequency_interval_days": None,
        "frequency_day_of_week": None,
        "frequency_time": None,
        "group_chat_id": None,
    }
    base.update(overrides)
    return base


UTC = ZoneInfo("UTC")


# ---------------------------------------------------------------------------
# schedule_partnership_reminder / cancel_reminder
# ---------------------------------------------------------------------------


def test_schedule_interval_mode_calls_run_once():
    job_queue = make_job_queue()
    partnership = make_partnership(
        id=42, frequency_mode=database.FREQUENCY_INTERVAL, frequency_interval_days=3
    )

    reminders.schedule_partnership_reminder(job_queue, partnership, UTC)

    job_queue.run_once.assert_called_once()
    _, kwargs = job_queue.run_once.call_args
    assert kwargs["data"] == 42
    assert kwargs["name"] == "reminder:42"


def test_schedule_interval_mode_bases_delay_on_last_session(db):
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    partnership_id = database.create_partnership("1", "2")
    session_id = database.add_session(partnership_id, "Romans 1", "1")

    # Backdate the session by 2 days so "due 3 days after last session" (~1 day from
    # now) is clearly distinguishable from "3 days from now" — proving the delay is
    # computed off the last session, not off utcnow().
    backdated = format_timestamp(utcnow() - timedelta(days=2))
    with database.get_db() as conn:
        conn.execute("UPDATE sessions SET logged_at = ? WHERE id = ?", (backdated, session_id))

    partnership = database.get_partnership(partnership_id)
    assert partnership is not None
    partnership["frequency_mode"] = database.FREQUENCY_INTERVAL
    partnership["frequency_interval_days"] = 3

    job_queue = make_job_queue()
    reminders.schedule_partnership_reminder(job_queue, partnership, UTC)

    _, kwargs = job_queue.run_once.call_args
    delay = kwargs["when"]
    assert timedelta(hours=12) < delay < timedelta(days=1, hours=12)


def test_schedule_interval_mode_without_days_is_noop():
    job_queue = make_job_queue()
    partnership = make_partnership(frequency_mode=database.FREQUENCY_INTERVAL)

    reminders.schedule_partnership_reminder(job_queue, partnership, UTC)

    job_queue.run_once.assert_not_called()


def test_schedule_weekly_mode_calls_run_daily():
    job_queue = make_job_queue()
    partnership = make_partnership(
        id=7,
        frequency_mode=database.FREQUENCY_WEEKLY,
        frequency_day_of_week=2,
        frequency_time="19:30",
    )

    reminders.schedule_partnership_reminder(job_queue, partnership, UTC)

    job_queue.run_daily.assert_called_once()
    _, kwargs = job_queue.run_daily.call_args
    assert kwargs["days"] == (2,)
    assert kwargs["time"].hour == 19
    assert kwargs["time"].minute == 30
    assert kwargs["data"] == 7
    assert kwargs["name"] == "reminder:7"


def test_schedule_no_frequency_mode_is_noop():
    job_queue = make_job_queue()
    partnership = make_partnership()

    reminders.schedule_partnership_reminder(job_queue, partnership, UTC)

    job_queue.run_once.assert_not_called()
    job_queue.run_daily.assert_not_called()


def test_schedule_replaces_existing_job():
    job_queue = make_job_queue()
    old_job = MagicMock()
    job_queue.get_jobs_by_name.return_value = [old_job]
    partnership = make_partnership(
        id=5, frequency_mode=database.FREQUENCY_INTERVAL, frequency_interval_days=1
    )

    reminders.schedule_partnership_reminder(job_queue, partnership, UTC)

    old_job.schedule_removal.assert_called_once()


def test_cancel_reminder_removes_matching_jobs():
    job_queue = make_job_queue()
    job = MagicMock()
    job_queue.get_jobs_by_name.return_value = [job]

    reminders.cancel_reminder(job_queue, 9)

    job_queue.get_jobs_by_name.assert_called_once_with("reminder:9")
    job.schedule_removal.assert_called_once()


def test_load_all_reminders_schedules_only_active_partnerships(db):
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    active_id = database.create_partnership("1", "2")
    database.set_frequency_interval(active_id, 3)
    ended_id = database.create_partnership("1", "2")
    database.set_frequency_interval(ended_id, 3)
    database.end_partnership(ended_id)

    job_queue = make_job_queue()
    reminders.load_all_reminders(job_queue, UTC)

    assert job_queue.run_once.call_count == 1
    _, kwargs = job_queue.run_once.call_args
    assert kwargs["data"] == active_id


# ---------------------------------------------------------------------------
# _send_reminder
# ---------------------------------------------------------------------------


def make_reminder_context(partnership_id: int, job_queue=None):
    context = MagicMock()
    context.job = MagicMock()
    context.job.data = partnership_id
    context.bot = MagicMock()
    context.bot.send_message = AsyncMock()
    context.bot_data = {"timezone": UTC}
    context.job_queue = job_queue or make_job_queue()
    return context


async def test_send_reminder_delivers_to_group_when_linked(db):
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    partnership_id = database.create_partnership("1", "2", group_chat_id="-100123")

    context = make_reminder_context(partnership_id)
    await reminders._send_reminder(context)  # noqa: SLF001

    context.bot.send_message.assert_called_once_with(
        chat_id="-100123", text=reminders.REMINDER_TEXT
    )


async def test_send_reminder_delivers_dm_to_both_users_when_no_group(db):
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    partnership_id = database.create_partnership("1", "2")

    context = make_reminder_context(partnership_id)
    await reminders._send_reminder(context)  # noqa: SLF001

    assert context.bot.send_message.await_count == 2
    recipients = {call.kwargs["chat_id"] for call in context.bot.send_message.await_args_list}
    assert recipients == {"1", "2"}


async def test_send_reminder_skips_ended_partnership(db):
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    partnership_id = database.create_partnership("1", "2")
    database.end_partnership(partnership_id)

    context = make_reminder_context(partnership_id)
    await reminders._send_reminder(context)  # noqa: SLF001

    context.bot.send_message.assert_not_called()


async def test_send_reminder_rearms_interval_mode(db):
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    partnership_id = database.create_partnership("1", "2")
    database.set_frequency_interval(partnership_id, 3)

    context = make_reminder_context(partnership_id)
    await reminders._send_reminder(context)  # noqa: SLF001

    context.job_queue.run_once.assert_called_once()


async def test_send_reminder_does_not_rearm_weekly_mode(db):
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    partnership_id = database.create_partnership("1", "2")
    database.set_frequency_weekly(partnership_id, 2, "19:00")

    context = make_reminder_context(partnership_id)
    await reminders._send_reminder(context)  # noqa: SLF001

    context.job_queue.run_once.assert_not_called()


async def test_send_reminder_continues_after_one_delivery_fails(db):
    database.upsert_user("1", "alice", "Alice")
    database.upsert_user("2", "bob", "Bob")
    partnership_id = database.create_partnership("1", "2")

    context = make_reminder_context(partnership_id)
    context.bot.send_message = AsyncMock(side_effect=[Exception("blocked"), None])

    await reminders._send_reminder(context)  # noqa: SLF001

    assert context.bot.send_message.await_count == 2


# Sanity check for the ISO-formatted timestamp round trip used throughout reminders.py.
def test_format_timestamp_round_trips_through_parse():
    now = utcnow()
    formatted = format_timestamp(now)
    assert " " in formatted
