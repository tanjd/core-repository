import pytest

from bookshelf_bot.config import Config


def test_from_env_reads_token_via_shared_helper(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("BOT_TOKEN", "prod-token")
    monkeypatch.delenv("ENV", raising=False)

    config = Config.from_env()

    assert config.telegram_bot_token == "prod-token"


def test_validate_raises_without_token() -> None:
    config = Config(telegram_bot_token="")

    with pytest.raises(ValueError, match="Bot token not configured"):
        config.validate()
