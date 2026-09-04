"""Entry point for running as python -m bookshelf_bot."""

import logging
import sys

from dotenv import load_dotenv
from telegram_bot_shared.health import start_health_server
from telegram_bot_shared.logging_setup import configure_logging

from bookshelf_bot.bot import build_application
from bookshelf_bot.config import Config

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

    start_health_server(config.health_check_port)

    app = build_application(config)
    logger.info("Starting bookshelf-bot...")
    app.run_polling(drop_pending_updates=True)


if __name__ == "__main__":
    main()
