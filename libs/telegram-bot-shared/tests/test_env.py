"""Tests for dev/prod bot-token selection."""

import pytest

from telegram_bot_shared.env import require_bot_token, select_bot_token


def test_select_bot_token_defaults_to_prod() -> None:
    assert select_bot_token({"BOT_TOKEN": "prod-token"}) == "prod-token"


def test_select_bot_token_uses_dev_token_when_env_is_dev() -> None:
    env = {"ENV": "dev", "BOT_TOKEN_DEV": "dev-token", "BOT_TOKEN": "prod-token"}
    assert select_bot_token(env) == "dev-token"


def test_select_bot_token_is_case_insensitive_and_trims_whitespace() -> None:
    env = {"ENV": " DEV ", "BOT_TOKEN_DEV": " dev-token "}
    assert select_bot_token(env) == "dev-token"


def test_select_bot_token_empty_when_unset() -> None:
    assert select_bot_token({}) == ""


def test_require_bot_token_returns_token_unchanged() -> None:
    assert require_bot_token("some-token") == "some-token"


def test_require_bot_token_raises_when_falsy() -> None:
    with pytest.raises(ValueError, match="Bot token not configured"):
        require_bot_token("")
