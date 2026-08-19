"""Entry point for running as python -m otobr_buddy."""

import logging
import sys
from zoneinfo import ZoneInfo

from dotenv import load_dotenv
from telegram_bot_shared.health import start_health_server
from telegram_bot_shared.logging_setup import configure_logging

from otobr_buddy import database, reminders
from otobr_buddy.bot import build_application
from otobr_buddy.config import Config

configure_logging(quiet_loggers=["httpx"])
logger = logging.getLogger(__name__)


def main() -> None:
    load_dotenv()
    config = Config.from_env()

    try:
        config.validate()
    except ValueError as e:
        logger.error("Configuration validation failed: %s", e)
        sys.exit(1)

    logger.info("Initializing database at %s", config.db_path)
    database.set_db_path(config.db_path)
    try:
        database.init_db()
    except Exception:
        logger.exception("Failed to initialize database")
        sys.exit(1)

    start_health_server(config.health_port)

    app = build_application(config)

    tz: ZoneInfo = app.bot_data["timezone"]
    assert app.job_queue is not None
    reminders.load_all_reminders(app.job_queue, tz)

    active = len(database.get_all_active_partnerships())
    logger.info("Starting bot; timezone=%s, active partnerships=%d", config.timezone, active)

    app.run_polling(drop_pending_updates=True)


if __name__ == "__main__":
    main()
