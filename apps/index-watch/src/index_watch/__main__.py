"""Entry point for running as python -m index_watch."""

import logging
import sys

from dotenv import load_dotenv

from index_watch import database
from index_watch.bot import alert_state, build_application, setup_scheduler
from index_watch.config import Config

logging.basicConfig(
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    level=logging.INFO,
    stream=sys.stdout,
)
logger = logging.getLogger(__name__)


def main() -> None:
    load_dotenv()
    config = Config.from_env()

    # Validate configuration
    try:
        config.validate()
    except ValueError as e:
        logger.error("Configuration validation failed: %s", e)
        sys.exit(1)

    # Initialize database
    logger.info("Initializing database at %s", config.db_path)
    try:
        database.init_db()
    except Exception as e:
        logger.error("Failed to initialize database: %s", e)
        sys.exit(1)

    # Check if volume is mounted (data directory is writable)
    data_dir = config.db_path.parent
    if not data_dir.exists() or not data_dir.is_dir():
        logger.warning(
            "Data directory %s does not exist! Data will not persist across restarts. "
            "Mount a volume to /app/data in your docker-compose.yml",
            data_dir,
        )

    # Migrate .env CHAT_IDS to database if database is empty
    active_subscribers = database.get_active_subscribers()
    if not active_subscribers and config.chat_ids:
        logger.info("Database is empty, migrating %d chat IDs from .env...", len(config.chat_ids))
        migrated = database.migrate_env_chat_ids(config.chat_ids)
        logger.info("Migrated %d chat IDs to database", migrated)
    elif config.chat_ids:
        logger.info(
            "Found %d active subscribers in database, %d in .env (using database)",
            len(active_subscribers),
            len(config.chat_ids),
        )
    else:
        logger.info("Found %d active subscribers in database", len(active_subscribers))

    # Load alert state from database
    try:
        persisted_state = database.load_alert_state()
        if persisted_state:
            alert_state.sent = persisted_state
            logger.info("Loaded %d alert states from database", len(persisted_state))
    except Exception as e:
        logger.warning("Failed to load alert state from database: %s", e)

    # Build and start bot
    app = build_application(config)
    setup_scheduler(app, config)

    db_stats = database.get_db_stats()
    logger.info(
        "Starting bot; daily report cron=%s, alert check every %s min, subscribers=%d (active=%d)",
        config.daily_report_cron,
        config.alert_check_minutes,
        db_stats["total_subscribers"],
        db_stats["active_subscribers"],
    )

    app.run_polling(drop_pending_updates=True)


if __name__ == "__main__":
    main()
