"""Tests for alert state logic."""

from index_watch.alerts import AlertState


def test_should_alert_first_time_below_threshold() -> None:
    state = AlertState()
    assert state.should_alert("^GSPC", 5, -6.0) is True
    assert state.should_alert("^GSPC", 10, -11.0) is True


def test_should_alert_false_above_threshold() -> None:
    state = AlertState()
    assert state.should_alert("^GSPC", 5, -3.0) is False
    assert state.should_alert("^GSPC", 5, 0.0) is False


def test_should_alert_false_after_mark_sent() -> None:
    state = AlertState()
    state.mark_sent("^GSPC", 5)
    assert state.should_alert("^GSPC", 5, -6.0) is False


def test_on_drawdown_improved_allows_alert_again() -> None:
    state = AlertState()
    state.mark_sent("^GSPC", 5)
    state.on_drawdown_improved("^GSPC", -3.0, (5, 10))
    assert state.should_alert("^GSPC", 5, -6.0) is True


def test_on_drawdown_improved_clears_only_improved_thresholds() -> None:
    """At -6% we are still below -5% so 5 stays sent; we improved past -10% so 10 is cleared."""
    state = AlertState()
    state.mark_sent("^GSPC", 5)
    state.mark_sent("^GSPC", 10)
    state.on_drawdown_improved("^GSPC", -6.0, (5, 10))
    assert ("^GSPC", 5) in state.sent
    assert ("^GSPC", 10) not in state.sent
