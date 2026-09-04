import httpx
import pytest

from bookshelf_bot.bot import LinkError, _confirm_link
from bookshelf_bot.config import Config


def _config() -> Config:
    return Config(
        telegram_bot_token="test-token",
        bookshelf_backend_url="http://backend.test",
        bookshelf_internal_token="secret",
    )


@pytest.mark.asyncio
async def test_confirm_link_success(monkeypatch: pytest.MonkeyPatch) -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.path == "/internal/telegram/confirm-link"
        assert request.headers["X-Internal-Secret"] == "secret"
        return httpx.Response(200, json={"name": "Ada"})

    _patch_client(monkeypatch, handler)

    name = await _confirm_link(_config(), "tok", 555)

    assert name == "Ada"


@pytest.mark.asyncio
async def test_confirm_link_expired_token_raises_with_detail(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    def handler(_request: httpx.Request) -> httpx.Response:
        return httpx.Response(400, json={"detail": "invalid or expired link"})

    _patch_client(monkeypatch, handler)

    with pytest.raises(LinkError, match="invalid or expired link"):
        await _confirm_link(_config(), "tok", 555)


@pytest.mark.asyncio
async def test_confirm_link_network_error_raises_generic_message(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    def handler(_request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("connection refused")

    _patch_client(monkeypatch, handler)

    with pytest.raises(LinkError, match="couldn't reach bookshelf"):
        await _confirm_link(_config(), "tok", 555)


def _patch_client(monkeypatch: pytest.MonkeyPatch, handler: object) -> None:
    """Redirects httpx.AsyncClient (as used inside _confirm_link) through a
    MockTransport, so tests never make a real network call.
    """
    transport = httpx.MockTransport(handler)  # type: ignore[arg-type]
    real_client = httpx.AsyncClient

    def fake_client(*args: object, **kwargs: object) -> httpx.AsyncClient:
        kwargs["transport"] = transport
        return real_client(*args, **kwargs)  # type: ignore[arg-type]

    monkeypatch.setattr("bookshelf_bot.bot.httpx.AsyncClient", fake_client)
