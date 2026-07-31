"""Application building and lifecycle management."""

import asyncio
import logging
from typing import Any

from telegram.ext import Application, CallbackQueryHandler, CommandHandler, ContextTypes

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
    CHAT_IDS_KEY,
    DEFAULT_BOT_VERSION,
    OFFLINE_MESSAGE,
)
from .handlers import (
    back_to_home,
    end_session,
    handle_exit,
    new_topic,
    next_card,
    previous_card,
    random_mix_chosen,
    show_bot_info,
    show_home,
    show_support,
    start,
    start_session,
    theme_chosen,
)

logger = logging.getLogger(__name__)

AppType = Application[Any, Any, Any, Any, Any, Any]


async def on_error(
    update: object,
    context: ContextTypes.DEFAULT_TYPE,
) -> None:
    """Handle errors in bot handlers."""
    err = context.error
    if err is None:
        return
    chat_id = context.bot_data.get("_last_chat_id") if hasattr(context, "bot_data") else None
    logger.exception("Handler error | chat_id=%s | error=%s", chat_id, err)


async def notify_going_offline(app: AppType) -> None:
    """Send 'going offline' to tracked chats with active sessions.

    Runs in post_stop so the bot is still usable.
    """
    raw = app.bot_data.get(CHAT_IDS_KEY)
    chat_ids: set[int] = raw if isinstance(raw, set) else set()  # type: ignore[assignment]
    if not chat_ids:
        logger.info("Shutdown: no chats to notify")
        return
    notified = 0
    skipped = 0
    for cid in chat_ids:
        # Check if chat has an active session
        chat_data = app.chat_data.get(cid)
        has_active_session = False
        if chat_data and isinstance(chat_data, dict):
            # Session is active if user has selected a theme or has questions loaded
            session_data: dict[str, object] = chat_data  # type: ignore[assignment]
            has_active_session = bool(
                session_data.get("theme_id") or session_data.get("shuffled_questions")
            )

        if not has_active_session:
            skipped += 1
            logger.debug("Shutdown: skipping chat_id=%s (no active session)", cid)
            continue

        try:
            await app.bot.send_message(chat_id=cid, text=OFFLINE_MESSAGE)
            notified += 1
        except Exception as e:
            logger.warning("Shutdown: failed to notify chat_id=%s: %s", cid, e)
    # Let the send requests flush before shutdown tears down the HTTP client
    if notified:
        await asyncio.sleep(1.0)
    logger.info(
        "Shutdown: notified %d chat(s) that bot is going offline "
        "(skipped %d without active sessions)",
        notified,
        skipped,
    )


def build_application(
    token: str,
    secret: str | None = None,
    env: str | None = None,
    creator_user_id: int | None = None,
    bot_version: str | None = None,
    changelog: str | None = None,
    coffee_link: str | None = None,
    deployment_time: str | None = None,
) -> AppType:
    """Build and configure the Telegram bot application."""
    app: AppType = (
        Application.builder()
        .token(token)
        .post_stop(notify_going_offline)  # type: ignore[arg-type]
        .build()
    )
    app.bot_data[CHAT_IDS_KEY] = set()
    if env:
        app.bot_data["env"] = env
        logger.info("Application built for %s environment", env)

    # Store home page configuration
    app.bot_data["creator_user_id"] = creator_user_id
    app.bot_data["bot_version"] = bot_version or DEFAULT_BOT_VERSION
    app.bot_data["changelog"] = changelog
    app.bot_data["coffee_link"] = coffee_link
    app.bot_data["deployment_time"] = deployment_time or "Unknown"
    # Register command handlers
    app.add_handler(CommandHandler("start", start))

    # Register existing callback handlers
    app.add_handler(CallbackQueryHandler(theme_chosen, pattern=f"^{CALLBACK_THEME_PREFIX}"))
    app.add_handler(CallbackQueryHandler(random_mix_chosen, pattern=f"^{CALLBACK_RANDOM_MIX}$"))
    app.add_handler(CallbackQueryHandler(next_card, pattern=f"^{CALLBACK_NEXT}$"))
    app.add_handler(CallbackQueryHandler(previous_card, pattern=f"^{CALLBACK_PREVIOUS}$"))
    app.add_handler(CallbackQueryHandler(new_topic, pattern=f"^{CALLBACK_NEW_TOPIC}$"))
    app.add_handler(CallbackQueryHandler(end_session, pattern=f"^{CALLBACK_END_SESSION}$"))

    # Register new home page callback handlers
    app.add_handler(CallbackQueryHandler(show_home, pattern=f"^{CALLBACK_HOME}$"))
    app.add_handler(CallbackQueryHandler(start_session, pattern=f"^{CALLBACK_START_SESSION}$"))
    app.add_handler(CallbackQueryHandler(show_bot_info, pattern=f"^{CALLBACK_BOT_INFO}$"))
    app.add_handler(CallbackQueryHandler(show_support, pattern=f"^{CALLBACK_SUPPORT}$"))
    app.add_handler(CallbackQueryHandler(handle_exit, pattern=f"^{CALLBACK_EXIT}$"))
    app.add_handler(CallbackQueryHandler(back_to_home, pattern=f"^{CALLBACK_BACK_TO_HOME}$"))

    app.add_error_handler(on_error)
    return app
