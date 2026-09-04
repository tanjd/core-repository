"""Telegram bot handlers."""

import logging

import httpx
from telegram import Update
from telegram.ext import Application, CommandHandler, ContextTypes

from bookshelf_bot.config import Config

logger = logging.getLogger(__name__)

_HELP_TEXT = (
    "I link your Telegram account to your bookshelf account so you get push "
    "notifications for loan requests, approvals, returns, and wishlist matches.\n\n"
    'To connect, tap "Connect Telegram" in your bookshelf profile settings — '
    "it opens a link here that finishes the setup automatically."
)


async def cmd_start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /start, with or without a link token.

    A bare /start (no args) happens when someone opens the bot directly
    rather than via the "Connect Telegram" deep link — greet them and point
    back to the web app, since there's nothing to link without a token.
    """
    if not update.message or not update.effective_chat:
        return

    if not context.args:
        await update.message.reply_text(_HELP_TEXT)
        return

    token = context.args[0]
    config: Config = context.bot_data["config"]

    try:
        name = await _confirm_link(config, token, update.effective_chat.id)
    except LinkError as e:
        logger.warning("telegram link failed: %s", e)
        await update.message.reply_text(
            f"Couldn't link your account: {e}. Go back to bookshelf and tap "
            '"Connect Telegram" again to get a fresh link.'
        )
        return

    await update.message.reply_text(f"Linked to {name}'s bookshelf account ✅")


async def cmd_help(update: Update, _context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /help."""
    if not update.message:
        return
    await update.message.reply_text(_HELP_TEXT)


class LinkError(Exception):
    """Raised when POST /internal/telegram/confirm-link fails."""


async def _confirm_link(config: Config, token: str, chat_id: int) -> str:
    """Calls bookshelf-backend's confirm-link endpoint. Returns the linked
    member's display name, or raises LinkError with a human-readable reason.
    """
    url = f"{config.bookshelf_backend_url}/internal/telegram/confirm-link"
    headers = {"X-Internal-Secret": config.bookshelf_internal_token}
    body = {"token": token, "chat_id": chat_id}

    async with httpx.AsyncClient(timeout=10.0) as client:
        try:
            resp = await client.post(url, json=body, headers=headers)
        except httpx.HTTPError as e:
            raise LinkError("couldn't reach bookshelf right now") from e

    if resp.status_code != httpx.codes.OK:
        detail = _error_detail(resp)
        raise LinkError(detail or "the link is invalid or has expired")

    return resp.json().get("name", "your")


def _error_detail(resp: httpx.Response) -> str:
    """Extracts huma's RFC 9457 "detail" field from an error response body,
    if present.
    """
    try:
        return str(resp.json().get("detail", ""))
    except ValueError:
        return ""


def build_application(config: Config) -> Application:
    """Create and configure the Telegram application."""
    app = Application.builder().token(config.telegram_bot_token).build()
    app.bot_data["config"] = config
    app.add_handler(CommandHandler("start", cmd_start))
    app.add_handler(CommandHandler("help", cmd_help))
    return app
