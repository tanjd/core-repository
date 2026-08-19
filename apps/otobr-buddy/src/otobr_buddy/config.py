"""Configuration from environment."""

import os
from dataclasses import dataclass, field
from pathlib import Path

from telegram_bot_shared.env import parse_admin_chat_ids, require_bot_token, select_bot_token

DEFAULT_TIMEZONE = "Asia/Singapore"  # GMT+8
DEFAULT_HEALTH_PORT = 9999


@dataclass
class Config:
    """Bot and data configuration."""

    telegram_bot_token: str = ""
    timezone: str = DEFAULT_TIMEZONE
    db_path: Path = field(default_factory=lambda: Path("data") / "otobr_buddy.db")
    health_port: int = DEFAULT_HEALTH_PORT
    admin_chat_ids: list[str] = field(default_factory=list)

    @classmethod
    def from_env(cls) -> "Config":
        """Load config from environment variables."""
        timezone = os.getenv("TIMEZONE", DEFAULT_TIMEZONE).strip() or DEFAULT_TIMEZONE
        db_path_str = os.getenv("DB_PATH", "data/otobr_buddy.db").strip()

        return cls(
            telegram_bot_token=select_bot_token(),
            timezone=timezone,
            db_path=Path(db_path_str),
            health_port=int(os.getenv("HEALTH_PORT", str(DEFAULT_HEALTH_PORT))),
            admin_chat_ids=parse_admin_chat_ids(),
        )

    def validate(self) -> None:
        """Validate configuration on startup."""
        require_bot_token(self.telegram_bot_token)

        if not 0 < self.health_port < 65536:
            raise ValueError("health_port must be a valid TCP port")
