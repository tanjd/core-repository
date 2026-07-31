"""Keyboard builders for the bot."""

import logging

from telegram import InlineKeyboardButton, InlineKeyboardMarkup

from ..data_loader import Theme, get_themes
from .constants import (
    CALLBACK_BACK_TO_HOME,
    CALLBACK_BOT_INFO,
    CALLBACK_END_SESSION,
    CALLBACK_EXIT,
    CALLBACK_HOME,
    CALLBACK_NEW_TOPIC,
    CALLBACK_NEXT,
    CALLBACK_PREVIOUS,
    CALLBACK_RANDOM_MIX,
    CALLBACK_START_SESSION,
    CALLBACK_SUPPORT,
    CALLBACK_THEME_PREFIX,
)

logger = logging.getLogger(__name__)

# Telegram's callback_data limit
MAX_CALLBACK_DATA_LENGTH = 64


def is_valid_callback_data(callback_data: str) -> bool:
    """Validate callback data meets Telegram's requirements.

    Args:
        callback_data: The callback data string to validate

    Returns:
        True if valid, False otherwise
    """
    if not callback_data:
        return False
    if len(callback_data.encode("utf-8")) > MAX_CALLBACK_DATA_LENGTH:
        return False
    # Check for problematic characters
    if any(c in callback_data for c in ["\n", "\r", "\t"]):
        return False
    return True


def theme_keyboard() -> InlineKeyboardMarkup:
    """Build keyboard with theme selection buttons and back to home button.

    Validates callback_data and skips themes with invalid IDs.
    """
    themes: list[Theme] = get_themes()
    buttons = []

    for t in themes:
        callback_data = f"{CALLBACK_THEME_PREFIX}{t['id']}"

        # Validate callback data
        if not is_valid_callback_data(callback_data):
            logger.error(
                f"Skipping theme '{t['label']}' - invalid callback_data: "
                f"{callback_data!r} ({len(callback_data.encode('utf-8'))} bytes)"
            )
            continue

        buttons.append([InlineKeyboardButton(t["label"], callback_data=callback_data)])

    # Add random mix button
    buttons.append([InlineKeyboardButton("🎲 Random Mix", callback_data=CALLBACK_RANDOM_MIX)])
    # Add back to home button at the bottom
    buttons.append([InlineKeyboardButton("🏠 Back to Home", callback_data=CALLBACK_BACK_TO_HOME)])
    return InlineKeyboardMarkup(buttons)


def home_keyboard() -> InlineKeyboardMarkup:
    """Build the home page keyboard with main menu options."""
    buttons = [
        [InlineKeyboardButton("🎯 Select Theme", callback_data=CALLBACK_START_SESSION)],
        [InlineKeyboardButton("ℹ️ Bot Info", callback_data=CALLBACK_BOT_INFO)],
        [InlineKeyboardButton("☕ Support Creator", callback_data=CALLBACK_SUPPORT)],
        [InlineKeyboardButton("❌ Exit", callback_data=CALLBACK_EXIT)],
    ]
    return InlineKeyboardMarkup(buttons)


def back_to_home_keyboard() -> InlineKeyboardMarkup:
    """Build keyboard with just a 'Back to Home' button."""
    buttons = [[InlineKeyboardButton("🏠 Back to Home", callback_data=CALLBACK_BACK_TO_HOME)]]
    return InlineKeyboardMarkup(buttons)


def navigation_keyboard(show_back: bool = True) -> InlineKeyboardMarkup:
    """Build keyboard with navigation and action buttons."""
    buttons: list[list[InlineKeyboardButton]] = []
    # Navigation row with back and next
    nav_row: list[InlineKeyboardButton] = []
    if show_back:
        nav_row.append(InlineKeyboardButton("← Back", callback_data=CALLBACK_PREVIOUS))
    nav_row.append(InlineKeyboardButton("Next →", callback_data=CALLBACK_NEXT))
    buttons.append(nav_row)
    # Other actions
    buttons.append([InlineKeyboardButton("🏠 Home", callback_data=CALLBACK_HOME)])
    buttons.append([InlineKeyboardButton("🔄 New Topic", callback_data=CALLBACK_NEW_TOPIC)])
    buttons.append([InlineKeyboardButton("🚪 End Session", callback_data=CALLBACK_END_SESSION)])
    return InlineKeyboardMarkup(buttons)
