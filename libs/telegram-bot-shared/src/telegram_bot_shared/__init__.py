"""Shared boilerplate for this repo's Telegram bots."""

from telegram_bot_shared.env import require_bot_token, select_bot_token
from telegram_bot_shared.health import start_health_server
from telegram_bot_shared.logging_setup import configure_logging

__all__ = [
    "configure_logging",
    "require_bot_token",
    "select_bot_token",
    "start_health_server",
]
