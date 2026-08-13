import pytest

from otobr_buddy.config import Config


def test_from_env_uses_prod_token_by_default(monkeypatch):
    monkeypatch.setenv("BOT_TOKEN", "prod-token")
    monkeypatch.setenv("BOT_TOKEN_DEV", "dev-token")
    monkeypatch.delenv("ENV", raising=False)

    config = Config.from_env()

    assert config.telegram_bot_token == "prod-token"


def test_from_env_uses_dev_token_when_env_is_dev(monkeypatch):
    monkeypatch.setenv("BOT_TOKEN", "prod-token")
    monkeypatch.setenv("BOT_TOKEN_DEV", "dev-token")
    monkeypatch.setenv("ENV", "dev")

    config = Config.from_env()

    assert config.telegram_bot_token == "dev-token"


def test_validate_requires_token():
    config = Config(telegram_bot_token="")

    with pytest.raises(ValueError, match="Bot token"):
        config.validate()


def test_validate_passes_with_token():
    config = Config(telegram_bot_token="some-token")

    config.validate()  # should not raise


def test_from_env_parses_admin_chat_ids(monkeypatch):
    monkeypatch.setenv("BOT_TOKEN", "prod-token")
    monkeypatch.setenv("ADMIN_CHAT_IDS", " 111, 222 ")

    config = Config.from_env()

    assert config.admin_chat_ids == ["111", "222"]


def test_from_env_admin_chat_ids_empty_by_default(monkeypatch):
    monkeypatch.setenv("BOT_TOKEN", "prod-token")
    monkeypatch.delenv("ADMIN_CHAT_IDS", raising=False)

    config = Config.from_env()

    assert config.admin_chat_ids == []
