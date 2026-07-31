"""Dev/prod bot-token selection from environment variables."""

import os
from collections.abc import Mapping


def select_bot_token(env: Mapping[str, str] | None = None) -> str:
    """Return BOT_TOKEN_DEV if ENV=dev, else BOT_TOKEN. Empty string if unset."""
    source = env if env is not None else os.environ
    active_env = (source.get("ENV") or "").strip().lower()
    token_dev = (source.get("BOT_TOKEN_DEV") or "").strip()
    token_prd = (source.get("BOT_TOKEN") or "").strip()
    return token_dev if active_env == "dev" else token_prd


def require_bot_token(token: str) -> str:
    """Return token unchanged, or raise ValueError if it's falsy."""
    if not token:
        raise ValueError("Bot token not configured (BOT_TOKEN or BOT_TOKEN_DEV required)")
    return token
