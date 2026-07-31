"""Session management and utilities."""

import logging
from typing import Any, TypedDict

from telegram import Update
from telegram.ext import Application, ContextTypes

logger = logging.getLogger(__name__)

AppType = Application[Any, Any, Any, Any, Any, Any]


class SessionDict(TypedDict, total=False):
    """Session state stored in context.chat_data (persisted per chat)."""

    theme_id: str | None
    index: int
    shuffled_questions: list[str]
    theme_labels: list[str] | None


def get_session(context: ContextTypes.DEFAULT_TYPE) -> SessionDict:
    """Get or initialize session data for the current chat."""
    if context.chat_data is None:
        raise RuntimeError("chat_data is required (PTB application default)")
    chat_data: SessionDict = context.chat_data  # type: ignore[assignment]
    if "theme_id" not in chat_data:
        chat_data["theme_id"] = None
        chat_data["index"] = 0
        chat_data["shuffled_questions"] = []
    return chat_data


def track_chat(application: AppType, update: Update) -> None:
    """Track chat IDs for shutdown notifications."""
    if update.effective_chat is None:
        return
    chat_id: int = update.effective_chat.id
    from .constants import CHAT_IDS_KEY

    chat_ids: set[int] = application.bot_data.setdefault(CHAT_IDS_KEY, set())
    chat_ids.add(chat_id)
    # for error handler context
    application.bot_data["_last_chat_id"] = chat_id


def log_action(update: Update, action: str, **extra: str | int) -> None:
    """Log bot actions with context."""
    chat_id = update.effective_chat.id if update.effective_chat else None
    user_id = update.effective_user.id if update.effective_user else None
    parts = [f"chat_id={chat_id}", f"user_id={user_id}", f"action={action}"]
    for k, v in extra.items():
        parts.append(f"{k}={v}")
    logger.info(" | ".join(parts))


def is_creator(update: Update, context: ContextTypes.DEFAULT_TYPE) -> bool:
    """Check if the current user is the bot creator."""
    if update.effective_user is None:
        return False
    app: AppType = context.application  # type: ignore[assignment]
    creator_id = app.bot_data.get("creator_user_id")
    if creator_id is None:
        return False
    return update.effective_user.id == creator_id


def format_card(questions: list[str], index: int, theme_labels: list[str] | None = None) -> str:
    """Format one question with 'Question N of M' and optional theme label."""
    total = len(questions)
    idx = index % total
    one_based = idx + 1

    # Add theme label if in random mix mode
    theme_info = ""
    if theme_labels and idx < len(theme_labels):
        theme_info = f"\n📚 Theme: {theme_labels[idx]}\n"

    return f"Question {one_based} of {total}{theme_info}\n{questions[idx]}"
