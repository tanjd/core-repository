"""Reminder scheduling for partnerships, backed by python-telegram-bot's JobQueue.

Interval-mode due dates are computed in naive UTC (matching how sqlite stores
timestamps) since only elapsed days matter. Weekly-mode jobs are scheduled with an
explicit tzinfo because "every Tuesday 7pm" is inherently about local wall-clock time.
"""

import logging
from datetime import time, timedelta
from typing import Any
from zoneinfo import ZoneInfo

from telegram.ext import ContextTypes, JobQueue

from otobr_buddy import database
from otobr_buddy.timeutil import parse_timestamp, utcnow

logger = logging.getLogger(__name__)

REMINDER_TEXT = (
    "\U0001f4d6 Time for your next one-to-one Bible reading session! Log it afterwards with /log."
)


def _job_name(partnership_id: int) -> str:
    return f"reminder:{partnership_id}"


def cancel_reminder(job_queue: JobQueue, partnership_id: int) -> None:
    """Remove any scheduled reminder job for a partnership."""
    for job in job_queue.get_jobs_by_name(_job_name(partnership_id)):
        job.schedule_removal()


def schedule_partnership_reminder(
    job_queue: JobQueue, partnership: dict[str, Any], tz: ZoneInfo
) -> None:
    """(Re)schedule the reminder job for a partnership based on its configured frequency.

    Safe to call repeatedly (e.g. after every /log) — it replaces any existing job.
    """
    cancel_reminder(job_queue, partnership["id"])

    mode = partnership.get("frequency_mode")
    if mode == database.FREQUENCY_INTERVAL:
        interval_days = partnership.get("frequency_interval_days")
        if not interval_days:
            return
        last_session = database.get_last_session(partnership["id"])
        from_dt = parse_timestamp(last_session["logged_at"]) if last_session else utcnow()
        due_at = from_dt + timedelta(days=interval_days)
        delay = max(due_at - utcnow(), timedelta(seconds=5))
        job_queue.run_once(
            _send_reminder,
            when=delay,
            data=partnership["id"],
            name=_job_name(partnership["id"]),
        )
    elif mode == database.FREQUENCY_WEEKLY:
        day_of_week = partnership.get("frequency_day_of_week")
        time_str = partnership.get("frequency_time")
        if day_of_week is None or not time_str:
            return
        hour, minute = (int(part) for part in time_str.split(":"))
        job_queue.run_daily(
            _send_reminder,
            time=time(hour=hour, minute=minute, tzinfo=tz),
            days=(day_of_week,),
            data=partnership["id"],
            name=_job_name(partnership["id"]),
        )


def load_all_reminders(job_queue: JobQueue, tz: ZoneInfo) -> None:
    """Schedule reminders for every active partnership (call once at startup)."""
    for partnership in database.get_all_active_partnerships():
        schedule_partnership_reminder(job_queue, partnership, tz)


async def _send_reminder(context: ContextTypes.DEFAULT_TYPE) -> None:
    assert context.job is not None
    partnership_id = context.job.data
    assert isinstance(partnership_id, int)
    partnership = database.get_partnership(partnership_id)
    if not partnership or partnership["status"] != database.STATUS_ACTIVE:
        return

    if partnership.get("group_chat_id"):
        try:
            await context.bot.send_message(chat_id=partnership["group_chat_id"], text=REMINDER_TEXT)
        except Exception:
            logger.exception("Failed to send reminder to group %s", partnership["group_chat_id"])
    else:
        for user_id in (partnership["user_a_id"], partnership["user_b_id"]):
            try:
                await context.bot.send_message(chat_id=user_id, text=REMINDER_TEXT)
            except Exception:
                logger.exception("Failed to send reminder to user %s", user_id)

    # Weekly jobs recur on their own; interval jobs are one-shot and must be re-armed.
    if partnership.get("frequency_mode") == database.FREQUENCY_INTERVAL:
        assert context.job_queue is not None
        tz: ZoneInfo = context.bot_data["timezone"]
        schedule_partnership_reminder(context.job_queue, partnership, tz)
