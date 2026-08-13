"""Dev/prod bot-token selection and admin-allowlist parsing from environment variables."""

import os
from collections.abc import Mapping, Sequence


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


def parse_admin_chat_ids(env: Mapping[str, str] | None = None) -> list[str]:
    """Parse ADMIN_CHAT_IDS (comma-separated) into a list of trimmed, non-empty IDs."""
    source = env if env is not None else os.environ
    raw = (source.get("ADMIN_CHAT_IDS") or "").strip()
    return [c.strip() for c in raw.split(",") if c.strip()]


def is_admin(chat_id: str, admin_chat_ids: Sequence[str]) -> bool:
    """True if chat_id may use admin-only commands.

    An empty admin_chat_ids list means no admin allowlist is configured, so the
    check is unrestricted (matches this repo's bots' existing opt-in behavior).
    """
    return not admin_chat_ids or chat_id in admin_chat_ids
