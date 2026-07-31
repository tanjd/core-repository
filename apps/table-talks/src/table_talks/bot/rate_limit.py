"""Rate limiting for bot handlers."""

import functools
import time
from collections.abc import Callable, Coroutine
from typing import Any

from telegram import Update
from telegram.ext import ContextTypes

# Rate limit configuration: (max_requests, window_seconds)
# More permissive limits for personal bot use
RATE_LIMITS = {
    "callback": (50, 10),
    "theme_selection": (10, 60),
    "card_navigation": (60, 60),
    "command": (20, 60),
}


def rate_limit(category: str):
    """Decorator to apply rate limiting to handlers."""

    def decorator(
        handler: Callable[[Update, ContextTypes.DEFAULT_TYPE], Coroutine[Any, Any, None]],
    ) -> Callable[[Update, ContextTypes.DEFAULT_TYPE], Coroutine[Any, Any, None]]:
        @functools.wraps(handler)
        async def wrapper(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
            if is_rate_limited(update, context, category):
                await handle_rate_limit_exceeded(update, context, category)
                return
            await handler(update, context)

        return wrapper

    return decorator


def is_rate_limited(_update: Update, context: ContextTypes.DEFAULT_TYPE, category: str) -> bool:
    """Check if user exceeded rate limit for category."""
    max_requests, window_seconds = RATE_LIMITS[category]

    # Get or initialize timestamps list
    if context.chat_data is None:
        return False
    timestamps: list[float] = context.chat_data.setdefault(f"rate_limit_{category}", [])

    # Clean old timestamps outside window
    current_time = time.time()
    cutoff = current_time - window_seconds
    timestamps[:] = [ts for ts in timestamps if ts > cutoff]

    # Check limit
    if len(timestamps) >= max_requests:
        return True

    # Record this request
    timestamps.append(current_time)
    return False


async def handle_rate_limit_exceeded(
    update: Update, context: ContextTypes.DEFAULT_TYPE, category: str
) -> None:
    """Handle rate limit exceeded with user-friendly message."""
    _, window_seconds = RATE_LIMITS[category]

    # Calculate cooldown time
    if context.chat_data is None:
        cooldown_seconds = window_seconds
    else:
        timestamps = context.chat_data.get(f"rate_limit_{category}", [])
        oldest_timestamp = min(timestamps) if timestamps else time.time()
        cooldown_seconds = int(window_seconds - (time.time() - oldest_timestamp))

    message = f"⏱️ Slow down! Please wait {cooldown_seconds} seconds before trying again."

    # Handle both callback queries and commands
    if update.callback_query:
        await update.callback_query.answer(message, show_alert=True)
    elif update.message:
        await update.message.reply_text(message)
