"""Tests for the shared logging setup helper."""

import logging

from telegram_bot_shared.logging_setup import configure_logging


def test_configure_logging_sets_root_level() -> None:
    configure_logging(level=logging.INFO, force=True)
    assert logging.getLogger().level == logging.INFO


def test_configure_logging_quiets_named_loggers() -> None:
    configure_logging(quiet_loggers=["httpx"], force=True)
    assert logging.getLogger("httpx").level == logging.WARNING
