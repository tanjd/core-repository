"""Bot command and callback handlers."""

import random
from typing import Any

from telegram import Update
from telegram.ext import Application, ContextTypes

from ..data_loader import Theme, get_questions, get_themes
from .constants import (
    BOT_INFO_MESSAGE,
    CALLBACK_THEME_PREFIX,
    DEFAULT_BOT_VERSION,
    EXIT_MESSAGE,
    HOME_WELCOME_MESSAGE,
    SUPPORT_CREATOR_MESSAGE,
)
from .keyboards import back_to_home_keyboard, home_keyboard, navigation_keyboard, theme_keyboard
from .rate_limit import rate_limit
from .session import format_card, get_session, log_action, track_chat

AppType = Application[Any, Any, Any, Any, Any, Any]


@rate_limit("command")
async def start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /start command."""
    if update.message is None:
        return
    app: AppType = context.application  # type: ignore[assignment]
    track_chat(app, update)
    session = get_session(context)
    session["theme_id"] = None
    session["index"] = 0
    session["shuffled_questions"] = []
    log_action(update, "start")
    await update.message.reply_text(
        HOME_WELCOME_MESSAGE,
        reply_markup=home_keyboard(),
    )


async def send_card(
    update: Update,
    context: ContextTypes.DEFAULT_TYPE,
    shuffled_questions: list[str],
    index: int,
    theme_labels: list[str] | None = None,
) -> None:
    """Send a question card to the user."""
    if not shuffled_questions:
        text = "No questions in this theme yet."
        markup = theme_keyboard()
    else:
        total = len(shuffled_questions)
        idx = index % total
        text = format_card(shuffled_questions, idx, theme_labels)
        # Show back button only if not on first question
        show_back = idx > 0
        markup = navigation_keyboard(show_back=show_back)
        session = get_session(context)
        session["index"] = idx + 1
        session["shuffled_questions"] = shuffled_questions
        if theme_labels:
            session["theme_labels"] = theme_labels
    if update.callback_query is not None:
        await update.callback_query.edit_message_text(text=text, reply_markup=markup)
    elif update.message is not None:
        await update.message.reply_text(text, reply_markup=markup)


@rate_limit("theme_selection")
async def theme_chosen(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle theme selection."""
    query = update.callback_query
    if query is None:
        return
    app: AppType = context.application  # type: ignore[assignment]
    track_chat(app, update)
    await query.answer()
    if not query.data or not query.data.startswith(CALLBACK_THEME_PREFIX):
        return
    theme_id = query.data[len(CALLBACK_THEME_PREFIX) :].strip()
    themes: list[Theme] = get_themes()
    if not any(t["id"] == theme_id for t in themes):
        log_action(update, "theme_chosen_invalid", theme_id=theme_id)
        return
    log_action(update, "theme_chosen", theme_id=theme_id)
    questions = get_questions(theme_id)
    shuffled: list[str] = questions.copy()
    random.shuffle(shuffled)
    session = get_session(context)
    session["theme_id"] = theme_id
    session["index"] = 0
    session["shuffled_questions"] = shuffled
    session["theme_labels"] = None  # Clear labels for single-theme mode
    await send_card(update, context, shuffled, 0)


@rate_limit("theme_selection")
async def random_mix_chosen(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle random mix selection - questions from all themes shuffled together."""
    query = update.callback_query
    if query is None:
        return
    app: AppType = context.application  # type: ignore[assignment]
    track_chat(app, update)
    await query.answer()

    log_action(update, "random_mix_chosen")

    # Get all questions with their theme IDs
    from ..data_loader import get_all_questions, get_themes

    all_questions = get_all_questions()

    # Shuffle questions
    shuffled_pairs = all_questions.copy()
    random.shuffle(shuffled_pairs)

    # Separate into parallel lists
    questions = [q for _, q in shuffled_pairs]
    theme_ids = [tid for tid, _ in shuffled_pairs]

    # Get theme labels for display
    themes = get_themes()
    theme_map = {t["id"]: t["label"] for t in themes}
    theme_labels = [theme_map.get(tid, "Unknown") for tid in theme_ids]

    # Store in session
    session = get_session(context)
    session["theme_id"] = "random_mix"
    session["index"] = 0
    session["shuffled_questions"] = questions
    session["theme_labels"] = theme_labels

    await send_card(update, context, questions, 0, theme_labels)


@rate_limit("card_navigation")
async def next_card(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle next card navigation."""
    query = update.callback_query
    if query is None:
        return
    app: AppType = context.application  # type: ignore[assignment]
    track_chat(app, update)
    await query.answer()
    session = get_session(context)
    shuffled = session.get("shuffled_questions", [])
    if not shuffled:
        log_action(update, "next_card_no_theme")
        await query.edit_message_text(
            "Choose a theme first.",
            reply_markup=theme_keyboard(),
        )
        return
    log_action(update, "next_card", theme_id=str(session.get("theme_id", "")))
    next_index = session.get("index", 0)
    theme_labels = session.get("theme_labels")
    await send_card(update, context, shuffled, next_index, theme_labels)


@rate_limit("card_navigation")
async def previous_card(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle previous card navigation."""
    query = update.callback_query
    if query is None:
        return
    app: AppType = context.application  # type: ignore[assignment]
    track_chat(app, update)
    await query.answer()
    session = get_session(context)
    shuffled = session.get("shuffled_questions", [])
    if not shuffled:
        log_action(update, "previous_card_no_theme")
        await query.edit_message_text(
            "Choose a theme first.",
            reply_markup=theme_keyboard(),
        )
        return
    current_index = session.get("index", 0)
    # Go back: current index points to next card, so go back 2 positions
    prev_index = max(0, current_index - 2)
    log_action(update, "previous_card", theme_id=str(session.get("theme_id", "")))
    theme_labels = session.get("theme_labels")
    await send_card(update, context, shuffled, prev_index, theme_labels)


@rate_limit("callback")
async def new_topic(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle new topic selection."""
    query = update.callback_query
    if query is None:
        return
    app: AppType = context.application  # type: ignore[assignment]
    track_chat(app, update)
    log_action(update, "new_topic")
    await query.answer()
    session = get_session(context)
    session["theme_id"] = None
    session["index"] = 0
    session["shuffled_questions"] = []
    await query.edit_message_text(
        "Choose a theme to get conversation cards. Each card is one question.",
        reply_markup=theme_keyboard(),
    )


@rate_limit("callback")
async def end_session(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle session end."""
    query = update.callback_query
    if query is None:
        return
    app: AppType = context.application  # type: ignore[assignment]
    track_chat(app, update)
    log_action(update, "end_session")
    await query.answer()
    session = get_session(context)
    session["theme_id"] = None
    session["index"] = 0
    session["shuffled_questions"] = []
    await query.edit_message_text(
        "Thanks for playing! Send /start to begin a new session.",
    )


@rate_limit("callback")
async def show_home(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Display the home page with main menu options."""
    app: AppType = context.application  # type: ignore[assignment]
    track_chat(app, update)

    # Clear theme session state when returning home
    session = get_session(context)
    session.pop("theme_id", None)
    session.pop("index", None)
    session.pop("shuffled_questions", None)

    log_action(update, "show_home")

    # Handle both command and callback contexts
    if update.callback_query is not None:
        await update.callback_query.answer()
        await update.callback_query.edit_message_text(
            text=HOME_WELCOME_MESSAGE,
            reply_markup=home_keyboard(),
        )
    elif update.message is not None:
        await update.message.reply_text(
            HOME_WELCOME_MESSAGE,
            reply_markup=home_keyboard(),
        )


@rate_limit("callback")
async def start_session(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle 'Start Session' button - show theme selection."""
    query = update.callback_query
    if query is None:
        return
    app: AppType = context.application  # type: ignore[assignment]
    track_chat(app, update)
    await query.answer()

    log_action(update, "start_session")

    # Clear session state
    session = get_session(context)
    session.pop("theme_id", None)
    session.pop("index", None)
    session.pop("shuffled_questions", None)

    await query.edit_message_text(
        text="Choose a theme to get conversation cards. Each card is one question.",
        reply_markup=theme_keyboard(),
    )


@rate_limit("callback")
async def show_bot_info(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Display bot information."""
    query = update.callback_query
    if query is None:
        return
    app: AppType = context.application  # type: ignore[assignment]
    track_chat(app, update)
    await query.answer()

    log_action(update, "show_bot_info")

    # Get config from bot_data
    version = app.bot_data.get("bot_version", DEFAULT_BOT_VERSION)
    last_updated = app.bot_data.get("deployment_time", "Unknown")
    changelog = app.bot_data.get("changelog", "Not available yet")

    message = BOT_INFO_MESSAGE.format(
        version=version,
        last_updated=last_updated,
        changelog=changelog,
    )

    await query.edit_message_text(
        text=message,
        reply_markup=back_to_home_keyboard(),
        parse_mode="Markdown",
    )


@rate_limit("callback")
async def show_support(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Show support information with coffee link."""
    query = update.callback_query
    if query is None:
        return
    app: AppType = context.application  # type: ignore[assignment]
    track_chat(app, update)
    await query.answer()

    log_action(update, "show_support")

    # Show coffee link to all users
    coffee_link = app.bot_data.get("coffee_link", "Not configured")
    message = SUPPORT_CREATOR_MESSAGE.format(coffee_link=coffee_link)

    await query.edit_message_text(
        text=message,
        reply_markup=back_to_home_keyboard(),
        parse_mode="Markdown",
    )


@rate_limit("callback")
async def handle_exit(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle exit button - clear session and show goodbye."""
    query = update.callback_query
    if query is None:
        return
    app: AppType = context.application  # type: ignore[assignment]
    track_chat(app, update)
    await query.answer()

    log_action(update, "handle_exit")

    # Clear all session state
    session = get_session(context)
    session.pop("theme_id", None)
    session.pop("index", None)
    session.pop("shuffled_questions", None)

    await query.edit_message_text(text=EXIT_MESSAGE)


@rate_limit("callback")
async def back_to_home(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Return to home page from any location."""
    await show_home(update, context)
