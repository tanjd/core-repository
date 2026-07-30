"""Tests for scheduled bot jobs: daily report and drawdown alert checks."""

from datetime import UTC, datetime
from unittest.mock import AsyncMock, MagicMock

import pytest

from index_watch import bot, database
from index_watch.alerts import AlertState, RecoveryState
from index_watch.config import Config
from index_watch.drawdown import DrawdownMetrics


@pytest.fixture(autouse=True)
def _reset_alert_state():
    bot.alert_state = AlertState()
    bot.recovery_state = RecoveryState()
    yield


def make_metrics(
    drawdown_pct: float = -2.0, price: float = 100.0, ath: float = 105.0
) -> DrawdownMetrics:
    return DrawdownMetrics(
        current_price=price,
        ath=ath,
        current_drawdown_pct=drawdown_pct,
        lowest_since_ath=price,
        drawdown_at_lowest_pct=drawdown_pct,
        gain_from_lowest_pct=0.0,
        gain_to_ath_from_current_pct=0.0,
        gain_to_ath_from_lowest_pct=0.0,
    )


def make_config(**overrides: object) -> Config:
    defaults: dict[str, object] = dict(
        telegram_bot_token="t",
        chat_ids=[],
        index_symbols={"^GSPC": "S&P 500"},
        drawdown_thresholds_pct=(5, 10),
    )
    defaults.update(overrides)
    return Config(**defaults)  # type: ignore[arg-type]


# -- _check_drawdown_alerts (sync core logic) --------------------------------


def test_check_drawdown_alerts_triggers_and_marks_sent(monkeypatch: pytest.MonkeyPatch) -> None:
    config = make_config()
    metrics = make_metrics(drawdown_pct=-6.0)
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    result = bot._check_drawdown_alerts(config, ["111"])
    assert list(result.keys()) == ["111"]
    assert len(result["111"]) == 1
    assert result["111"][0].name == "S&P 500"
    assert result["111"][0].threshold_pct == 5
    assert ("111", "^GSPC", 5) in bot.alert_state.sent


def test_check_drawdown_alerts_skips_stale_data(monkeypatch: pytest.MonkeyPatch) -> None:
    config = make_config()
    metrics = make_metrics(drawdown_pct=-6.0)
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), True)
    )
    result = bot._check_drawdown_alerts(config, ["111"])
    assert result == {}


def test_check_drawdown_alerts_no_metrics_available(monkeypatch: pytest.MonkeyPatch) -> None:
    config = make_config()
    monkeypatch.setattr(bot, "get_index_metrics", lambda symbol, name, years: None)
    result = bot._check_drawdown_alerts(config, ["111"])
    assert result == {}


def test_check_drawdown_alerts_does_not_repeat_same_threshold(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    config = make_config()
    metrics = make_metrics(drawdown_pct=-6.0)
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    bot._check_drawdown_alerts(config, ["111"])
    result = bot._check_drawdown_alerts(config, ["111"])
    assert result == {}


def test_check_drawdown_alerts_fans_out_to_all_subscribers(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    config = make_config()
    metrics = make_metrics(drawdown_pct=-6.0)
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    result = bot._check_drawdown_alerts(config, ["111", "222"])
    assert set(result.keys()) == {"111", "222"}


def test_check_drawdown_alerts_respects_per_subscriber_threshold_override(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """111 has a custom threshold of 10% only, so a -6% drawdown must not alert them."""
    config = make_config()
    database.set_subscriber_thresholds("111", [10])
    metrics = make_metrics(drawdown_pct=-6.0)
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    result = bot._check_drawdown_alerts(config, ["111", "222"])
    assert "111" not in result
    assert "222" in result
    assert result["222"][0].threshold_pct == 5


def test_check_drawdown_alerts_respects_per_subscriber_index_override(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """111 narrows to ^NDX only, so alerts on ^GSPC must not reach them."""
    config = make_config(index_symbols={"^GSPC": "S&P 500", "^NDX": "NASDAQ-100"})
    database.set_subscriber_indices("111", ["^NDX"])
    metrics = make_metrics(drawdown_pct=-6.0)
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    result = bot._check_drawdown_alerts(config, ["111", "222"])
    assert {d.symbol for d in result["111"]} == {"^NDX"}
    assert {d.symbol for d in result["222"]} == {"^GSPC", "^NDX"}


# -- _check_recovery_notifications (sync core logic) -------------------------


def test_check_recovery_notifications_notifies_on_recovery(monkeypatch: pytest.MonkeyPatch) -> None:
    config = make_config()
    metrics = make_metrics(drawdown_pct=0.0, price=105.0, ath=105.0)
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    result = bot._check_recovery_notifications(config, ["111"])
    assert len(result["111"]) == 1
    assert result["111"][0].symbol == "^GSPC"
    assert result["111"][0].is_new_ath is True  # no prior tracked ATH -> treated as new
    assert ("111", "^GSPC") in bot.recovery_state.notified


def test_check_recovery_notifications_no_notify_while_in_drawdown(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    config = make_config()
    metrics = make_metrics(drawdown_pct=-3.0)
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    result = bot._check_recovery_notifications(config, ["111"])
    assert result == {}


def test_check_recovery_notifications_does_not_repeat(monkeypatch: pytest.MonkeyPatch) -> None:
    config = make_config()
    metrics = make_metrics(drawdown_pct=0.0, price=105.0, ath=105.0)
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    bot._check_recovery_notifications(config, ["111"])
    result = bot._check_recovery_notifications(config, ["111"])
    assert result == {}


def test_check_recovery_notifications_distinguishes_new_vs_previous_ath(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    config = make_config()
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )

    # Cycle 1: recovers to ATH of 100 (first time seeing this symbol -> treated as new).
    metrics_cycle1 = make_metrics(drawdown_pct=0.0, price=100.0, ath=100.0)
    monkeypatch.setattr(
        bot,
        "get_index_metrics",
        lambda symbol, name, years: (metrics_cycle1, datetime.now(UTC), False),
    )
    result1 = bot._check_recovery_notifications(config, ["111"])
    assert result1["111"][0].is_new_ath is True

    # Cycle 2: dips then re-recovers to the SAME ath (100) -> genuinely a "previous" ATH now.
    bot.recovery_state.on_drawdown_worsened("111", "^GSPC", -1.0)
    metrics_cycle2 = make_metrics(drawdown_pct=0.0, price=100.0, ath=100.0)
    monkeypatch.setattr(
        bot,
        "get_index_metrics",
        lambda symbol, name, years: (metrics_cycle2, datetime.now(UTC), False),
    )
    result2 = bot._check_recovery_notifications(config, ["111"])
    assert result2["111"][0].is_new_ath is False


# -- check_and_send_alerts (async job) ---------------------------------------


async def test_check_and_send_alerts_sends_and_updates_stats(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    config = make_config()
    database.add_subscriber("111")
    metrics = make_metrics(drawdown_pct=-6.0)
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    app = MagicMock()
    app.bot.send_message = AsyncMock()
    await bot.check_and_send_alerts(app, config)
    app.bot.send_message.assert_called_once()
    stats = database.get_subscriber_stats("111")
    assert stats is not None
    assert stats["last_alert_sent"] is not None
    assert database.load_alert_state() == {("111", "^GSPC", 5)}


async def test_check_and_send_alerts_batches_multiple_triggers_into_one_message(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Two thresholds crossing at once for one subscriber must still be a single send."""
    config = make_config()
    database.add_subscriber("111")
    metrics = make_metrics(drawdown_pct=-12.0)  # crosses both 5% and 10% thresholds
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    app = MagicMock()
    app.bot.send_message = AsyncMock()
    await bot.check_and_send_alerts(app, config)
    app.bot.send_message.assert_called_once()
    text = app.bot.send_message.call_args.kwargs["text"]
    assert "Drawdown Alerts (2)" in text


async def test_check_and_send_alerts_no_subscribers_sends_nothing() -> None:
    config = make_config()
    app = MagicMock()
    app.bot.send_message = AsyncMock()
    await bot.check_and_send_alerts(app, config)
    app.bot.send_message.assert_not_called()


async def test_check_and_send_alerts_falls_back_to_env_chat_ids(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    config = make_config(chat_ids=["999"])
    metrics = make_metrics(drawdown_pct=-6.0)
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    app = MagicMock()
    app.bot.send_message = AsyncMock()
    await bot.check_and_send_alerts(app, config)
    assert app.bot.send_message.call_args.kwargs["chat_id"] == "999"


async def test_check_and_send_alerts_saves_state_even_with_no_alerts(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    config = make_config()
    database.add_subscriber("111")
    metrics = make_metrics(drawdown_pct=-1.0)
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    app = MagicMock()
    app.bot.send_message = AsyncMock()
    await bot.check_and_send_alerts(app, config)
    app.bot.send_message.assert_not_called()
    assert database.load_alert_state() == set()


async def test_check_and_send_alerts_one_send_failure_does_not_abort_others(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    config = make_config()
    database.add_subscriber("111")
    database.add_subscriber("222")
    metrics = make_metrics(drawdown_pct=-6.0)
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    app = MagicMock()

    async def send_message(chat_id: str, text: str, parse_mode: str | None = None) -> None:
        if chat_id == "111":
            raise RuntimeError("telegram down")

    app.bot.send_message = AsyncMock(side_effect=send_message)
    await bot.check_and_send_alerts(app, config)
    assert app.bot.send_message.call_count == 2
    stats_222 = database.get_subscriber_stats("222")
    assert stats_222 is not None
    assert stats_222["last_alert_sent"] is not None
    stats_111 = database.get_subscriber_stats("111")
    assert stats_111 is not None
    assert stats_111["last_alert_sent"] is None


async def test_check_and_send_alerts_sends_recovery_only_digest(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    config = make_config()
    database.add_subscriber("111")
    metrics = make_metrics(drawdown_pct=0.0, price=105.0, ath=105.0)
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    app = MagicMock()
    app.bot.send_message = AsyncMock()
    await bot.check_and_send_alerts(app, config)
    app.bot.send_message.assert_called_once()
    text = app.bot.send_message.call_args.kwargs["text"]
    assert "Recovery / New Highs" in text
    assert "Drawdown Alert" not in text
    assert database.load_recovery_state() == {("111", "^GSPC")}


async def test_check_and_send_alerts_combines_alerts_and_recoveries_one_message(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """A subscriber tracking two indices, one dropping and one recovering, gets one message."""
    config = make_config(index_symbols={"^GSPC": "S&P 500", "^NDX": "NASDAQ-100"})
    database.add_subscriber("111")

    def fake_metrics(symbol: str, name: str, years: int):
        if symbol == "^GSPC":
            return (make_metrics(drawdown_pct=-6.0), datetime.now(UTC), False)
        return (make_metrics(drawdown_pct=0.0, price=105.0, ath=105.0), datetime.now(UTC), False)

    monkeypatch.setattr(bot, "get_index_metrics", fake_metrics)
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    app = MagicMock()
    app.bot.send_message = AsyncMock()
    await bot.check_and_send_alerts(app, config)
    app.bot.send_message.assert_called_once()
    text = app.bot.send_message.call_args.kwargs["text"]
    assert "Drawdown Alert" in text
    assert "Recovery / New Highs" in text


# -- send_daily_report / _build_daily_report ---------------------------------


async def test_send_daily_report_sends_to_subscribers(monkeypatch: pytest.MonkeyPatch) -> None:
    config = make_config()
    database.add_subscriber("111")
    monkeypatch.setattr(bot, "_build_daily_report", lambda _config: "report")
    app = MagicMock()
    app.bot.send_message = AsyncMock()
    await bot.send_daily_report(app, config)
    app.bot.send_message.assert_called_once()
    stats = database.get_subscriber_stats("111")
    assert stats is not None
    assert stats["last_daily_sent"] is not None


async def test_send_daily_report_no_subscribers_sends_nothing(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    config = make_config()
    monkeypatch.setattr(bot, "_build_daily_report", lambda _config: "report")
    app = MagicMock()
    app.bot.send_message = AsyncMock()
    await bot.send_daily_report(app, config)
    app.bot.send_message.assert_not_called()


def test_build_daily_report_includes_stale_warning(monkeypatch: pytest.MonkeyPatch) -> None:
    config = make_config()
    metrics = make_metrics()
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), True)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), True)
    )
    monkeypatch.setattr(bot, "fetch_fear_greed", lambda: None)
    report = bot._build_daily_report(config)
    assert "may be outdated" in report


def test_build_daily_report_no_stale_warning_when_fresh(monkeypatch: pytest.MonkeyPatch) -> None:
    config = make_config()
    metrics = make_metrics()
    monkeypatch.setattr(
        bot, "get_index_metrics", lambda symbol, name, years: (metrics, datetime.now(UTC), False)
    )
    monkeypatch.setattr(
        bot, "fetch_index_history", lambda symbol, years: ([100.0] * 10, datetime.now(UTC), False)
    )
    monkeypatch.setattr(bot, "fetch_fear_greed", lambda: None)
    report = bot._build_daily_report(config)
    assert "may be outdated" not in report
    assert "S&P 500" in report
