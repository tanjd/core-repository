"""Tests for config loading."""

import pytest

from index_watch.config import DEFAULT_DRAWDOWN_THRESHOLDS, DEFAULT_INDEX_SYMBOLS, Config


def test_config_from_env_defaults(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("ENV", raising=False)
    monkeypatch.delenv("BOT_TOKEN", raising=False)
    monkeypatch.delenv("BOT_TOKEN_DEV", raising=False)
    monkeypatch.delenv("TELEGRAM_CHAT_IDS", raising=False)
    monkeypatch.delenv("DRAWDOWN_THRESHOLDS_PCT", raising=False)
    monkeypatch.delenv("DAILY_REPORT_CRON", raising=False)
    monkeypatch.delenv("ALERT_CHECK_MINUTES", raising=False)
    monkeypatch.delenv("HISTORY_YEARS", raising=False)
    config = Config.from_env()
    assert config.telegram_bot_token == ""
    assert config.chat_ids == []
    assert config.drawdown_thresholds_pct == DEFAULT_DRAWDOWN_THRESHOLDS
    assert config.history_years == 20


def test_config_from_env_token_dev(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("BOT_TOKEN", raising=False)
    monkeypatch.setenv("BOT_TOKEN_DEV", "dev-token")
    monkeypatch.setenv("ENV", "dev")
    config = Config.from_env()
    assert config.telegram_bot_token == "dev-token"


def test_config_from_env_token_prd(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("BOT_TOKEN_DEV", raising=False)
    monkeypatch.setenv("BOT_TOKEN", "prd-token")
    monkeypatch.setenv("ENV", "prd")
    config = Config.from_env()
    assert config.telegram_bot_token == "prd-token"


def test_config_from_env_default_env_uses_prd_token(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("ENV", raising=False)
    monkeypatch.setenv("BOT_TOKEN", "prd-token")
    monkeypatch.delenv("BOT_TOKEN_DEV", raising=False)
    config = Config.from_env()
    assert config.telegram_bot_token == "prd-token"


def test_config_from_env_chat_ids(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("BOT_TOKEN", raising=False)
    monkeypatch.delenv("BOT_TOKEN_DEV", raising=False)
    monkeypatch.setenv("TELEGRAM_CHAT_IDS", "123, 456 ")
    config = Config.from_env()
    assert config.chat_ids == ["123", "456"]


def test_config_from_env_thresholds(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("BOT_TOKEN", raising=False)
    monkeypatch.delenv("BOT_TOKEN_DEV", raising=False)
    monkeypatch.setenv("DRAWDOWN_THRESHOLDS_PCT", "5 10 15 20 25")
    config = Config.from_env()
    assert config.drawdown_thresholds_pct == (5, 10, 15, 20, 25)


def test_config_from_env_index_symbols_default(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("INDEX_SYMBOLS", raising=False)
    config = Config.from_env()
    assert config.index_symbols == DEFAULT_INDEX_SYMBOLS


def test_config_from_env_index_symbols_single_pair(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("INDEX_SYMBOLS", "^GSPC:S&P 500")
    config = Config.from_env()
    assert config.index_symbols == {"^GSPC": "S&P 500"}


def test_config_from_env_index_symbols_multiple_pairs(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("INDEX_SYMBOLS", "^GSPC:S&P 500;^NDX:NASDAQ-100")
    config = Config.from_env()
    assert config.index_symbols == {"^GSPC": "S&P 500", "^NDX": "NASDAQ-100"}


def test_config_from_env_index_symbols_missing_display_name_falls_back_to_symbol(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("INDEX_SYMBOLS", "^GSPC")
    config = Config.from_env()
    assert config.index_symbols == {"^GSPC": "^GSPC"}


def test_config_from_env_index_symbols_skips_blank_entries(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("INDEX_SYMBOLS", "^GSPC:S&P 500;;  ;^NDX:NASDAQ-100")
    config = Config.from_env()
    assert config.index_symbols == {"^GSPC": "S&P 500", "^NDX": "NASDAQ-100"}


def test_config_validate_rejects_empty_index_symbols() -> None:
    config = Config(telegram_bot_token="t", index_symbols={})
    with pytest.raises(ValueError, match="index symbol"):
        config.validate()
