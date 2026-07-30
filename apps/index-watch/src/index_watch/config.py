"""Configuration from environment."""

import os
from dataclasses import dataclass, field
from pathlib import Path

DEFAULT_INDEX_SYMBOLS = {"^GSPC": "S&P 500", "^NDX": "NASDAQ-100"}
DEFAULT_DRAWDOWN_THRESHOLDS = (5, 10, 15, 20)
DEFAULT_DAILY_REPORT_CRON = "0 22 * * 1-5"  # 22:00 UTC Mon–Fri (after US close)
DEFAULT_ALERT_CHECK_MINUTES = 60  # Increased from 30 to reduce API calls
DEFAULT_DISPLAY_TIMEZONE = "Asia/Singapore"  # GMT+8
DEFAULT_CACHE_TTL_SECONDS = 30 * 60  # 30 minutes cache TTL


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
    history_years: int = 30
    display_timezone: str = DEFAULT_DISPLAY_TIMEZONE
    db_path: Path = field(default_factory=lambda: Path("data") / "index_watch.db")
    cache_ttl_seconds: int = DEFAULT_CACHE_TTL_SECONDS

    @classmethod
    def from_env(cls) -> "Config":
        """Load config from environment variables."""
        env = os.getenv("ENV", "").strip().lower()
        token_dev = (os.getenv("BOT_TOKEN_DEV") or "").strip()
        token_prd = (os.getenv("BOT_TOKEN") or "").strip()
        token = token_dev if env == "dev" else token_prd

        raw_chat_ids = os.getenv("TELEGRAM_CHAT_IDS", "").strip()
        chat_ids = [c.strip() for c in raw_chat_ids.split(",") if c.strip()]

        raw_admin_ids = os.getenv("ADMIN_CHAT_IDS", "").strip()
        admin_chat_ids = [c.strip() for c in raw_admin_ids.split(",") if c.strip()]

        raw_thresholds = os.getenv("DRAWDOWN_THRESHOLDS_PCT", "").strip()
        if raw_thresholds:
            thresholds = tuple(int(x) for x in raw_thresholds.replace("%", "").split())
        else:
            thresholds = DEFAULT_DRAWDOWN_THRESHOLDS

        display_tz = os.getenv("DISPLAY_TIMEZONE", DEFAULT_DISPLAY_TIMEZONE).strip()
        db_path_str = os.getenv("DB_PATH", "data/index_watch.db").strip()

        return cls(
            telegram_bot_token=token,
            chat_ids=chat_ids,
            admin_chat_ids=admin_chat_ids,
            drawdown_thresholds_pct=thresholds,
            daily_report_cron=os.getenv("DAILY_REPORT_CRON", DEFAULT_DAILY_REPORT_CRON).strip()
            or DEFAULT_DAILY_REPORT_CRON,
            alert_check_minutes=int(
                os.getenv("ALERT_CHECK_MINUTES", str(DEFAULT_ALERT_CHECK_MINUTES))
            ),
            history_years=int(os.getenv("HISTORY_YEARS", "20")),
            display_timezone=display_tz or DEFAULT_DISPLAY_TIMEZONE,
            db_path=Path(db_path_str),
            cache_ttl_seconds=int(os.getenv("CACHE_TTL_SECONDS", str(DEFAULT_CACHE_TTL_SECONDS))),
        )

    def validate(self) -> None:
        """Validate configuration on startup."""
        if not self.telegram_bot_token:
            raise ValueError("Bot token not configured (BOT_TOKEN or BOT_TOKEN_DEV required)")

        if not 0 < self.alert_check_minutes < 1440:
            raise ValueError("alert_check_minutes must be between 1 and 1440")

        if not self.drawdown_thresholds_pct:
            raise ValueError("At least one drawdown threshold required")

        for t in self.drawdown_thresholds_pct:
            if not 0 < t < 100:
                raise ValueError(f"Threshold {t} must be between 0 and 100")

        if self.history_years < 1:
            raise ValueError("history_years must be at least 1")
