"""Rate limiting utilities for bot commands."""

import logging
from collections import defaultdict
from datetime import UTC, datetime, timedelta

logger = logging.getLogger(__name__)


class RateLimiter:
    """Simple per-user rate limiter with configurable cooldowns."""

    def __init__(self):
        """Initialize rate limiter with empty state."""
        self._last_request: dict[str, dict[str, datetime]] = defaultdict(dict)

    def check_rate_limit(self, user_id: str, command: str, cooldown_seconds: int) -> int | None:
        """
        Check if user is rate limited for a command.

        Args:
            user_id: Telegram chat ID
            command: Command name (e.g., "daily", "subscribe")
            cooldown_seconds: Cooldown period in seconds

        Returns:
            None if allowed, or remaining seconds until next allowed request
        """
        now = datetime.now(UTC)
        last_time = self._last_request[user_id].get(command)

        if last_time:
            elapsed = (now - last_time).total_seconds()
            if elapsed < cooldown_seconds:
                remaining = int(cooldown_seconds - elapsed)
                logger.info(
                    "Rate limit hit: user=%s command=%s remaining=%ds",
                    user_id,
                    command,
                    remaining,
                )
                return remaining

        # Update timestamp
        self._last_request[user_id][command] = now
        logger.debug("Rate limit passed: user=%s command=%s", user_id, command)
        return None

    def reset_user(self, user_id: str) -> None:
        """Reset rate limit for a specific user."""
        if user_id in self._last_request:
            del self._last_request[user_id]
            logger.info("Rate limit reset for user=%s", user_id)

    def cleanup_old_entries(self, max_age_hours: int = 24) -> None:
        """Remove entries older than max_age_hours to prevent memory bloat."""
        now = datetime.now(UTC)
        cutoff = now - timedelta(hours=max_age_hours)
        users_to_remove = []

        for user_id, commands in self._last_request.items():
            # Remove old command entries
            commands_to_remove = [cmd for cmd, timestamp in commands.items() if timestamp < cutoff]
            for cmd in commands_to_remove:
                del commands[cmd]

            # If user has no recent commands, mark for removal
            if not commands:
                users_to_remove.append(user_id)

        for user_id in users_to_remove:
            del self._last_request[user_id]

        if users_to_remove:
            logger.info("Cleaned up rate limiter: removed %d users", len(users_to_remove))


# Rate limit configurations (command -> cooldown in seconds)
RATE_LIMITS = {
    "daily": 5 * 60,  # 5 minutes
    "subscribe": 60,  # 1 minute
    "unsubscribe": 60,  # 1 minute
    "status": 10,  # 10 seconds
    "mystats": 10,  # 10 seconds
    "alerts": 10,  # 10 seconds
    "debug": 60,  # 1 minute
    "clearcache": 30,  # 30 seconds
}
