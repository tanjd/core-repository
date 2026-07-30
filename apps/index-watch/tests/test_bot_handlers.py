"""Tests for Telegram command handlers in bot.py."""

from unittest.mock import AsyncMock, MagicMock

import pytest

from index_watch import bot, database
from index_watch.alerts import AlertState
from index_watch.cache import get_cache
from index_watch.config import Config
from index_watch.rate_limiter import RateLimiter


@pytest.fixture(autouse=True)
def _reset_bot_singletons():
    """bot.alert_state/rate_limiter/cache are module-level singletons; isolate each test."""
    bot.alert_state = AlertState()
    bot.rate_limiter = RateLimiter()
    get_cache().clear()
    yield
    get_cache().clear()


def make_update(chat_id: str = "123", username: str | None = "alice") -> MagicMock:
    update = MagicMock()
    update.message = MagicMock()
    update.message.chat_id = int(chat_id)
    update.message.reply_text = AsyncMock()
    if username is not None:
        user = MagicMock()
        user.username = username
        update.message.from_user = user
    else:
        update.message.from_user = None
    return update


def make_context(
    config: Config | None = None,
    scheduler: object | None = None,
    args: list[str] | None = None,
) -> MagicMock:
    context = MagicMock()
    context.bot_data = {}
    context.args = args or []
    if config is not None:
        context.bot_data["config"] = config
    if scheduler is not None:
        context.bot_data["scheduler"] = scheduler
    return context


def reply_text(update: MagicMock) -> str:
    return update.message.reply_text.call_args.args[0]


async def test_cmd_start() -> None:
    update = make_update()
    await bot.cmd_start(update, make_context())
    assert "Index Watch" in reply_text(update)


async def test_cmd_subscribe_new() -> None:
    update = make_update(chat_id="123")
    await bot.cmd_subscribe(update, make_context())
    assert "subscribed" in reply_text(update).lower()
    assert database.is_subscribed("123") is True


async def test_cmd_subscribe_already_subscribed() -> None:
    database.add_subscriber("123")
    update = make_update(chat_id="123")
    await bot.cmd_subscribe(update, make_context())
    assert "already subscribed" in reply_text(update).lower()


async def test_cmd_subscribe_rate_limited() -> None:
    update = make_update(chat_id="123")
    context = make_context()
    await bot.cmd_subscribe(update, context)
    update.message.reply_text.reset_mock()
    await bot.cmd_subscribe(update, context)
    assert "wait" in reply_text(update).lower()


async def test_cmd_subscribe_db_error(monkeypatch: pytest.MonkeyPatch) -> None:
    def raise_error(*_args: object, **_kwargs: object) -> None:
        raise RuntimeError("db down")

    monkeypatch.setattr(database, "add_subscriber", raise_error)
    update = make_update(chat_id="123")
    await bot.cmd_subscribe(update, make_context())
    assert "failed" in reply_text(update).lower()


async def test_cmd_unsubscribe_success() -> None:
    database.add_subscriber("123")
    update = make_update(chat_id="123")
    await bot.cmd_unsubscribe(update, make_context())
    assert "unsubscribed" in reply_text(update).lower()
    assert database.is_subscribed("123") is False


async def test_cmd_unsubscribe_not_subscribed() -> None:
    update = make_update(chat_id="123")
    await bot.cmd_unsubscribe(update, make_context())
    assert "not currently subscribed" in reply_text(update).lower()


async def test_cmd_unsubscribe_rate_limited() -> None:
    database.add_subscriber("123")
    update = make_update(chat_id="123")
    context = make_context()
    await bot.cmd_unsubscribe(update, context)
    update.message.reply_text.reset_mock()
    await bot.cmd_unsubscribe(update, context)
    assert "wait" in reply_text(update).lower()


async def test_cmd_status_not_subscribed() -> None:
    update = make_update(chat_id="123")
    await bot.cmd_status(update, make_context())
    assert "Not subscribed" in reply_text(update)


async def test_cmd_status_subscribed_with_config_and_scheduler() -> None:
    database.add_subscriber("123")
    scheduler = MagicMock()
    job = MagicMock()
    job.next_run_time = "2026-08-01 22:00"
    scheduler.get_job.return_value = job
    config = Config(telegram_bot_token="t")
    update = make_update(chat_id="123")
    await bot.cmd_status(update, make_context(config=config, scheduler=scheduler))
    text = reply_text(update)
    assert "Subscribed" in text
    assert "Alert thresholds" in text


async def test_cmd_mystats_not_subscribed() -> None:
    update = make_update(chat_id="123")
    await bot.cmd_mystats(update, make_context())
    assert "not subscribed" in reply_text(update).lower()


async def test_cmd_mystats_subscribed() -> None:
    database.add_subscriber("123")
    update = make_update(chat_id="123")
    await bot.cmd_mystats(update, make_context())
    assert "Subscription Stats" in reply_text(update)


async def test_cmd_daily_no_config() -> None:
    update = make_update(chat_id="123")
    await bot.cmd_daily(update, make_context())
    assert "starting up" in reply_text(update).lower()


async def test_cmd_daily_success(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(bot, "_build_daily_report", lambda _config: "report text")
    config = Config(telegram_bot_token="t")
    update = make_update(chat_id="123")
    await bot.cmd_daily(update, make_context(config=config))
    assert reply_text(update) == "report text"


async def test_cmd_daily_rate_limited(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(bot, "_build_daily_report", lambda _config: "report text")
    config = Config(telegram_bot_token="t")
    update = make_update(chat_id="123")
    context = make_context(config=config)
    await bot.cmd_daily(update, context)
    update.message.reply_text.reset_mock()
    await bot.cmd_daily(update, context)
    assert "wait" in reply_text(update).lower()


async def test_cmd_daily_failure(monkeypatch: pytest.MonkeyPatch) -> None:
    def raise_error(_config: Config) -> str:
        raise RuntimeError("yfinance down")

    monkeypatch.setattr(bot, "_build_daily_report", raise_error)
    config = Config(telegram_bot_token="t")
    update = make_update(chat_id="123")
    await bot.cmd_daily(update, make_context(config=config))
    assert "Failed to fetch" in reply_text(update)


async def test_cmd_alerts_with_config() -> None:
    config = Config(telegram_bot_token="t")
    update = make_update(chat_id="123")
    await bot.cmd_alerts(update, make_context(config=config))
    assert "5%" in reply_text(update)


async def test_cmd_alerts_no_config() -> None:
    update = make_update(chat_id="123")
    await bot.cmd_alerts(update, make_context())
    assert "Config not loaded" in reply_text(update)


async def test_cmd_debug_non_admin_rejected() -> None:
    config = Config(telegram_bot_token="t", admin_chat_ids=["999"])
    scheduler = MagicMock()
    update = make_update(chat_id="123")
    await bot.cmd_debug(update, make_context(config=config, scheduler=scheduler))
    assert "restricted" in reply_text(update).lower()


async def test_cmd_debug_admin_allowed() -> None:
    config = Config(telegram_bot_token="t", admin_chat_ids=["123"])
    scheduler = MagicMock()
    scheduler.running = True
    scheduler.get_jobs.return_value = []
    update = make_update(chat_id="123")
    await bot.cmd_debug(update, make_context(config=config, scheduler=scheduler))
    assert "Debug Info" in reply_text(update)


async def test_cmd_debug_no_admin_restriction_allows_anyone() -> None:
    config = Config(telegram_bot_token="t", admin_chat_ids=[])
    scheduler = MagicMock()
    scheduler.running = True
    scheduler.get_jobs.return_value = []
    update = make_update(chat_id="123")
    await bot.cmd_debug(update, make_context(config=config, scheduler=scheduler))
    assert "Debug Info" in reply_text(update)


async def test_cmd_debug_no_scheduler() -> None:
    config = Config(telegram_bot_token="t")
    update = make_update(chat_id="123")
    await bot.cmd_debug(update, make_context(config=config))
    assert "Scheduler not initialized" in reply_text(update)


async def test_cmd_clearcache_admin() -> None:
    config = Config(telegram_bot_token="t", admin_chat_ids=["123"])
    get_cache().set("k", "v", 60)
    update = make_update(chat_id="123")
    await bot.cmd_clearcache(update, make_context(config=config))
    assert "Cache cleared" in reply_text(update)


async def test_cmd_clearcache_non_admin_rejected() -> None:
    config = Config(telegram_bot_token="t", admin_chat_ids=["999"])
    update = make_update(chat_id="123")
    await bot.cmd_clearcache(update, make_context(config=config))
    assert "restricted" in reply_text(update).lower()


# -- /mysettings, /setthresholds, /myindices, /setindices --------------------


async def test_cmd_mysettings_shows_defaults_when_no_override() -> None:
    config = Config(telegram_bot_token="t")
    update = make_update(chat_id="123")
    await bot.cmd_mysettings(update, make_context(config=config))
    text = reply_text(update)
    assert "(default)" in text
    assert "5%, 10%, 15%, 20%" in text


async def test_cmd_mysettings_shows_custom_when_overridden() -> None:
    config = Config(telegram_bot_token="t")
    database.set_subscriber_thresholds("123", [5])
    database.set_subscriber_indices("123", ["^GSPC"])
    update = make_update(chat_id="123")
    await bot.cmd_mysettings(update, make_context(config=config))
    text = reply_text(update)
    assert "(custom)" in text
    assert "5%" in text


async def test_cmd_setthresholds_no_args_shows_usage() -> None:
    update = make_update(chat_id="123")
    await bot.cmd_setthresholds(update, make_context())
    assert "usage" in reply_text(update).lower()


async def test_cmd_setthresholds_sets_override() -> None:
    update = make_update(chat_id="123")
    await bot.cmd_setthresholds(update, make_context(args=["5", "10"]))
    assert database.get_subscriber_thresholds("123") == (5, 10)
    assert "5%, 10%" in reply_text(update)


async def test_cmd_setthresholds_default_clears_override() -> None:
    database.set_subscriber_thresholds("123", [5])
    update = make_update(chat_id="123")
    await bot.cmd_setthresholds(update, make_context(args=["default"]))
    assert database.get_subscriber_thresholds("123") is None


async def test_cmd_setthresholds_rejects_non_numeric() -> None:
    update = make_update(chat_id="123")
    await bot.cmd_setthresholds(update, make_context(args=["abc"]))
    assert "whole numbers" in reply_text(update)
    assert database.get_subscriber_thresholds("123") is None


async def test_cmd_setthresholds_rejects_out_of_range() -> None:
    update = make_update(chat_id="123")
    await bot.cmd_setthresholds(update, make_context(args=["150"]))
    assert "between 1 and 99" in reply_text(update)
    assert database.get_subscriber_thresholds("123") is None


async def test_cmd_myindices_shows_default_selection() -> None:
    config = Config(telegram_bot_token="t")
    update = make_update(chat_id="123")
    await bot.cmd_myindices(update, make_context(config=config))
    text = reply_text(update)
    assert "Using all default indices" in text
    assert "✅" in text


async def test_cmd_myindices_shows_custom_selection() -> None:
    config = Config(telegram_bot_token="t")
    database.set_subscriber_indices("123", ["^GSPC"])
    update = make_update(chat_id="123")
    await bot.cmd_myindices(update, make_context(config=config))
    text = reply_text(update)
    assert "Custom selection" in text
    assert "⬜️" in text  # NASDAQ-100 and MSCI World are unselected


async def test_cmd_setindices_no_args_shows_usage() -> None:
    config = Config(telegram_bot_token="t")
    update = make_update(chat_id="123")
    await bot.cmd_setindices(update, make_context(config=config))
    assert "usage" in reply_text(update).lower()


async def test_cmd_setindices_sets_override() -> None:
    config = Config(telegram_bot_token="t")
    update = make_update(chat_id="123")
    await bot.cmd_setindices(update, make_context(config=config, args=["^GSPC"]))
    assert database.get_subscriber_indices("123") == ("^GSPC",)
    assert "S&P 500" in reply_text(update)


async def test_cmd_setindices_rejects_unknown_symbol() -> None:
    config = Config(telegram_bot_token="t")
    update = make_update(chat_id="123")
    await bot.cmd_setindices(update, make_context(config=config, args=["^BOGUS"]))
    assert "Unknown symbol" in reply_text(update)
    assert database.get_subscriber_indices("123") is None


async def test_cmd_setindices_default_clears_override() -> None:
    config = Config(telegram_bot_token="t")
    database.set_subscriber_indices("123", ["^GSPC"])
    update = make_update(chat_id="123")
    await bot.cmd_setindices(update, make_context(config=config, args=["default"]))
    assert database.get_subscriber_indices("123") is None
