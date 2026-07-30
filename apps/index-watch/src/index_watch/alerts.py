"""Drawdown threshold alert state and logic."""

from dataclasses import dataclass, field


@dataclass
class AlertDetail:
    """One triggered drawdown-threshold alert for a single index/subscriber."""

    symbol: str
    name: str
    current_drawdown_pct: float
    threshold_pct: int
    day_count: int
    total_days: int
    history_years: int


@dataclass
class AlertState:
    """Tracks which (chat_id, symbol, threshold) alerts have been sent per subscriber."""

    sent: set[tuple[str, str, int]] = field(default_factory=set)

    def should_alert(
        self, chat_id: str, symbol: str, threshold_pct: int, current_drawdown_pct: float
    ) -> bool:
        """True if we should send an alert: drawdown at or beyond threshold and not yet sent."""
        if current_drawdown_pct > -threshold_pct:
            return False
        return (chat_id, symbol, threshold_pct) not in self.sent

    def mark_sent(self, chat_id: str, symbol: str, threshold_pct: int) -> None:
        self.sent.add((chat_id, symbol, threshold_pct))

    def on_drawdown_improved(
        self,
        chat_id: str,
        symbol: str,
        current_drawdown_pct: float,
        thresholds: tuple[int, ...],
    ) -> None:
        """When drawdown improves above a threshold, allow alerting again."""
        to_remove = [
            (c, s, t)
            for (c, s, t) in self.sent
            if c == chat_id and s == symbol and current_drawdown_pct > -t
        ]
        for key in to_remove:
            self.sent.discard(key)


@dataclass
class RecoveryDetail:
    """One recovery/new-ATH notification for a single index/subscriber."""

    symbol: str
    name: str
    current_price: float
    ath: float
    is_new_ath: bool


@dataclass
class RecoveryState:
    """
    Tracks per-subscriber recovery/new-ATH notifications.

    `notified` (per chat_id+symbol) prevents repeat notifications until the
    index dips back into a drawdown. `last_ath` (per symbol, shared across all
    subscribers - it's a market fact, not a subscriber setting) is used to tell
    a genuinely new all-time high apart from merely recovering to a previously
    seen one. `last_ath` is in-memory only (not persisted): after a bot restart
    the first notification for an index already at/near its ATH may be labeled
    "new" when it was actually a prior high - a minor, low-consequence quirk,
    not worth extra persistence for.
    """

    notified: set[tuple[str, str]] = field(default_factory=set)
    last_ath: dict[str, float] = field(default_factory=dict)

    def should_notify(self, chat_id: str, symbol: str, current_drawdown_pct: float) -> bool:
        """True if the index has recovered (drawdown >= 0) and not already notified."""
        if current_drawdown_pct < 0:
            return False
        return (chat_id, symbol) not in self.notified

    def mark_notified(self, chat_id: str, symbol: str) -> None:
        self.notified.add((chat_id, symbol))

    def on_drawdown_worsened(self, chat_id: str, symbol: str, current_drawdown_pct: float) -> None:
        """Once back in a drawdown, allow notifying again on the next recovery."""
        if current_drawdown_pct < 0:
            self.notified.discard((chat_id, symbol))

    def is_new_ath(self, symbol: str, current_ath: float) -> bool:
        """True if current_ath is higher than the last ATH tracked for this symbol."""
        previous = self.last_ath.get(symbol)
        return previous is None or current_ath > previous

    def update_ath(self, symbol: str, current_ath: float) -> None:
        self.last_ath[symbol] = current_ath
