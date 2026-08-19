"""Tests for dev/prod bot-token selection."""

import pytest

from telegram_bot_shared.env import (
    is_admin,
    parse_admin_chat_ids,
    require_bot_token,
    select_bot_token,
)


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


def test_parse_admin_chat_ids_splits_and_trims() -> None:
    env = {"ADMIN_CHAT_IDS": " 123, 456 ,789"}
    assert parse_admin_chat_ids(env) == ["123", "456", "789"]


def test_parse_admin_chat_ids_empty_when_unset() -> None:
    assert parse_admin_chat_ids({}) == []


def test_is_admin_unrestricted_when_no_admins_configured() -> None:
    assert is_admin("123", []) is True


def test_is_admin_true_for_listed_chat_id() -> None:
    assert is_admin("123", ["123", "456"]) is True


def test_is_admin_false_for_unlisted_chat_id() -> None:
    assert is_admin("999", ["123", "456"]) is False
