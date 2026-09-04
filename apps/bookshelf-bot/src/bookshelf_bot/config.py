"""Configuration from environment."""

import os
from dataclasses import dataclass

from telegram_bot_shared.env import require_bot_token, select_bot_token


@dataclass
class Config:
    """Bot configuration."""

    telegram_bot_token: str = ""
    health_check_port: int = 8080
    # bookshelf_backend_url/internal_token: this bot's one job is completing
    # the "Connect Telegram" deep link (see
    # apps/bookshelf/docs/telegram-bot-integration-spec.md) by calling
    # bookshelf-backend's POST /internal/telegram/confirm-link. That
    # endpoint isn't user-facing — internal_token must match the backend's
    # own TELEGRAM_INTERNAL_SECRET.
    bookshelf_backend_url: str = ""
    bookshelf_internal_token: str = ""

    @classmethod
    def from_env(cls) -> Config:
        """Load config from environment variables."""
        return cls(
            telegram_bot_token=select_bot_token(),
            health_check_port=int(os.getenv("HEALTH_CHECK_PORT", "8080")),
            bookshelf_backend_url=os.getenv("BOOKSHELF_BACKEND_URL", "").rstrip("/"),
            bookshelf_internal_token=os.getenv("BOOKSHELF_INTERNAL_TOKEN", ""),
        )

    def validate(self) -> None:
        """Validate configuration on startup."""
        require_bot_token(self.telegram_bot_token)
        if not self.bookshelf_backend_url:
            raise ValueError("BOOKSHELF_BACKEND_URL not configured")
        if not self.bookshelf_internal_token:
            raise ValueError("BOOKSHELF_INTERNAL_TOKEN not configured")
