"""Tests for recovery/new-ATH notification state logic."""

from index_watch.alerts import RecoveryState


def test_should_notify_true_when_recovered() -> None:
    state = RecoveryState()
    assert state.should_notify("111", "^GSPC", 0.0) is True
    assert state.should_notify("111", "^GSPC", 1.0) is True


def test_should_notify_false_while_in_drawdown() -> None:
    state = RecoveryState()
    assert state.should_notify("111", "^GSPC", -0.5) is False


def test_should_notify_false_after_mark_notified() -> None:
    state = RecoveryState()
    state.mark_notified("111", "^GSPC")
    assert state.should_notify("111", "^GSPC", 0.0) is False


def test_should_notify_independent_per_subscriber() -> None:
    state = RecoveryState()
    state.mark_notified("111", "^GSPC")
    assert state.should_notify("222", "^GSPC", 0.0) is True


def test_on_drawdown_worsened_allows_notify_again() -> None:
    state = RecoveryState()
    state.mark_notified("111", "^GSPC")
    state.on_drawdown_worsened("111", "^GSPC", -1.0)
    assert state.should_notify("111", "^GSPC", 0.0) is True


def test_on_drawdown_worsened_noop_when_still_recovered() -> None:
    state = RecoveryState()
    state.mark_notified("111", "^GSPC")
    state.on_drawdown_worsened("111", "^GSPC", 0.5)  # not worsened
    assert state.should_notify("111", "^GSPC", 0.0) is False


def test_on_drawdown_worsened_scoped_to_subscriber() -> None:
    state = RecoveryState()
    state.mark_notified("111", "^GSPC")
    state.mark_notified("222", "^GSPC")
    state.on_drawdown_worsened("111", "^GSPC", -1.0)
    assert state.should_notify("111", "^GSPC", 0.0) is True
    assert state.should_notify("222", "^GSPC", 0.0) is False


def test_is_new_ath_true_when_no_prior_ath_tracked() -> None:
    state = RecoveryState()
    assert state.is_new_ath("^GSPC", 100.0) is True


def test_is_new_ath_true_when_higher_than_tracked() -> None:
    state = RecoveryState()
    state.update_ath("^GSPC", 100.0)
    assert state.is_new_ath("^GSPC", 105.0) is True


def test_is_new_ath_false_when_equal_to_tracked() -> None:
    state = RecoveryState()
    state.update_ath("^GSPC", 100.0)
    assert state.is_new_ath("^GSPC", 100.0) is False


def test_is_new_ath_false_when_lower_than_tracked() -> None:
    state = RecoveryState()
    state.update_ath("^GSPC", 100.0)
    assert state.is_new_ath("^GSPC", 95.0) is False


def test_update_ath_scoped_per_symbol() -> None:
    state = RecoveryState()
    state.update_ath("^GSPC", 100.0)
    assert state.is_new_ath("^NDX", 50.0) is True
