"""Tests for alert state logic."""

from index_watch.alerts import AlertState


def test_should_alert_first_time_below_threshold() -> None:
    state = AlertState()
    assert state.should_alert("111", "^GSPC", 5, -6.0) is True
    assert state.should_alert("111", "^GSPC", 10, -11.0) is True


def test_should_alert_false_above_threshold() -> None:
    state = AlertState()
    assert state.should_alert("111", "^GSPC", 5, -3.0) is False
    assert state.should_alert("111", "^GSPC", 5, 0.0) is False


def test_should_alert_false_after_mark_sent() -> None:
    state = AlertState()
    state.mark_sent("111", "^GSPC", 5)
    assert state.should_alert("111", "^GSPC", 5, -6.0) is False


def test_should_alert_independent_per_subscriber() -> None:
    """Marking sent for one subscriber must not suppress alerts for another."""
    state = AlertState()
    state.mark_sent("111", "^GSPC", 5)
    assert state.should_alert("222", "^GSPC", 5, -6.0) is True


def test_on_drawdown_improved_allows_alert_again() -> None:
    state = AlertState()
    state.mark_sent("111", "^GSPC", 5)
    state.on_drawdown_improved("111", "^GSPC", -3.0, (5, 10))
    assert state.should_alert("111", "^GSPC", 5, -6.0) is True


def test_on_drawdown_improved_clears_only_improved_thresholds() -> None:
    """At -6% we are still below -5% so 5 stays sent; we improved past -10% so 10 is cleared."""
    state = AlertState()
    state.mark_sent("111", "^GSPC", 5)
    state.mark_sent("111", "^GSPC", 10)
    state.on_drawdown_improved("111", "^GSPC", -6.0, (5, 10))
    assert ("111", "^GSPC", 5) in state.sent
    assert ("111", "^GSPC", 10) not in state.sent


def test_on_drawdown_improved_scoped_to_subscriber() -> None:
    """Improving drawdown for one subscriber must not clear another subscriber's sent state."""
    state = AlertState()
    state.mark_sent("111", "^GSPC", 5)
    state.mark_sent("222", "^GSPC", 5)
    state.on_drawdown_improved("111", "^GSPC", -3.0, (5,))
    assert ("111", "^GSPC", 5) not in state.sent
    assert ("222", "^GSPC", 5) in state.sent
