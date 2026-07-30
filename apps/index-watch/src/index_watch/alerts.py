"""Drawdown threshold alert state and logic."""

from dataclasses import dataclass, field


@dataclass
class AlertState:
    """Tracks which (symbol, threshold) alerts have been sent so we don't repeat until recovery."""

    sent: set[tuple[str, int]] = field(default_factory=set)

    def should_alert(self, symbol: str, threshold_pct: int, current_drawdown_pct: float) -> bool:
        """True if we should send an alert: drawdown at or beyond threshold and not yet sent."""
        if current_drawdown_pct > -threshold_pct:
            return False
        return (symbol, threshold_pct) not in self.sent

    def mark_sent(self, symbol: str, threshold_pct: int) -> None:
        self.sent.add((symbol, threshold_pct))

    def on_drawdown_improved(
        self, symbol: str, current_drawdown_pct: float, thresholds: tuple[int, ...]
    ) -> None:
        """When drawdown improves above a threshold, allow alerting again."""
        to_remove = [(s, t) for (s, t) in self.sent if s == symbol and current_drawdown_pct > -t]
        for key in to_remove:
            self.sent.discard(key)
