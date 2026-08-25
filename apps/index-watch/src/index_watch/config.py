"""Configuration from environment."""

import os
from dataclasses import dataclass, field
from pathlib import Path

from telegram_bot_shared.env import parse_admin_chat_ids, require_bot_token, select_bot_token

DEFAULT_INDEX_SYMBOLS = {
    "^GSPC": "S&P 500",
    "^NDX": "NASDAQ-100",
    "^990100-USD-STRD": "MSCI World",
}
DEFAULT_DRAWDOWN_THRESHOLDS = (5, 10, 15, 20)
DEFAULT_DAILY_REPORT_CRON = "0 22 * * 1-5"  # 22:00 UTC Mon–Fri (after US close)
DEFAULT_ALERT_CHECK_MINUTES = 60  # Increased from 30 to reduce API calls
DEFAULT_HISTORY_YEARS = 20
DEFAULT_DISPLAY_TIMEZONE = "Asia/Singapore"  # GMT+8
DEFAULT_CACHE_TTL_SECONDS = 30 * 60  # 30 minutes cache TTL


def _parse_index_symbols(raw: str) -> dict[str, str]:
    """Parse 'symbol:Display Name;symbol:Display Name' pairs; empty falls back to defaults."""
    raw = raw.strip()
    if not raw:
        return dict(DEFAULT_INDEX_SYMBOLS)

    symbols: dict[str, str] = {}
    for pair in raw.split(";"):
        pair = pair.strip()
        if not pair:
            continue
        symbol, _sep, name = pair.partition(":")
        symbol = symbol.strip()
        name = name.strip()
        if not symbol:
            continue
        symbols[symbol] = name or symbol
    return symbols


@dataclass
class Config:
    """Bot and data configuration."""

    telegram_bot_token: str = ""
    chat_ids: list[str] = field(default_factory=list)
    admin_chat_ids: list[str] = field(default_factory=list)
    index_symbols: dict[str, str] = field(default_factory=lambda: dict(DEFAULT_INDEX_SYMBOLS))
    drawdown_thresholds_pct: tuple[int, ...] = DEFAULT_DRAWDOWN_THRESHOLDS
    daily_report_cron: str = DEFAULT_DAILY_REPORT_CRON
    alert_check_minutes: int = DEFAULT_ALERT_CHECK_MINUTES
    history_years: int = DEFAULT_HISTORY_YEARS
    display_timezone: str = DEFAULT_DISPLAY_TIMEZONE
    db_path: Path = field(default_factory=lambda: Path("data") / "index_watch.db")
    cache_ttl_seconds: int = DEFAULT_CACHE_TTL_SECONDS
    health_check_port: int = 8080

    @classmethod
    def from_env(cls) -> Config:
        """Load config from environment variables."""
        token = select_bot_token()

        raw_chat_ids = os.getenv("TELEGRAM_CHAT_IDS", "").strip()
        chat_ids = [c.strip() for c in raw_chat_ids.split(",") if c.strip()]

        admin_chat_ids = parse_admin_chat_ids()

        raw_thresholds = os.getenv("DRAWDOWN_THRESHOLDS_PCT", "").strip()
        if raw_thresholds:
            thresholds = tuple(int(x) for x in raw_thresholds.replace("%", "").split())
        else:
            thresholds = DEFAULT_DRAWDOWN_THRESHOLDS

        index_symbols = _parse_index_symbols(os.getenv("INDEX_SYMBOLS", ""))

        display_tz = os.getenv("DISPLAY_TIMEZONE", DEFAULT_DISPLAY_TIMEZONE).strip()
        db_path_str = os.getenv("DB_PATH", "data/index_watch.db").strip()

        return cls(
            telegram_bot_token=token,
            chat_ids=chat_ids,
            admin_chat_ids=admin_chat_ids,
            index_symbols=index_symbols,
            drawdown_thresholds_pct=thresholds,
            daily_report_cron=os.getenv("DAILY_REPORT_CRON", DEFAULT_DAILY_REPORT_CRON).strip()
            or DEFAULT_DAILY_REPORT_CRON,
            alert_check_minutes=int(
                os.getenv("ALERT_CHECK_MINUTES", str(DEFAULT_ALERT_CHECK_MINUTES))
            ),
            history_years=int(os.getenv("HISTORY_YEARS", str(DEFAULT_HISTORY_YEARS))),
            display_timezone=display_tz or DEFAULT_DISPLAY_TIMEZONE,
            db_path=Path(db_path_str),
            cache_ttl_seconds=int(os.getenv("CACHE_TTL_SECONDS", str(DEFAULT_CACHE_TTL_SECONDS))),
            health_check_port=int(os.getenv("HEALTH_CHECK_PORT", "8080")),
        )

    def validate(self) -> None:
        """Validate configuration on startup."""
        require_bot_token(self.telegram_bot_token)

        if not 0 < self.alert_check_minutes < 1440:
            raise ValueError("alert_check_minutes must be between 1 and 1440")

        if not self.index_symbols:
            raise ValueError("At least one index symbol required (check INDEX_SYMBOLS)")

        if not self.drawdown_thresholds_pct:
            raise ValueError("At least one drawdown threshold required")

        for t in self.drawdown_thresholds_pct:
            if not 0 < t < 100:
                raise ValueError(f"Threshold {t} must be between 0 and 100")

        if self.history_years < 1:
            raise ValueError("history_years must be at least 1")
