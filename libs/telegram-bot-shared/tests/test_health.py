"""Tests for the shared health-check server."""

import urllib.error
import urllib.request

from telegram_bot_shared.health import start_health_server


def _get(port: int, path: str) -> tuple[int, bytes]:
    try:
        with urllib.request.urlopen(f"http://127.0.0.1:{port}{path}", timeout=5) as response:
            return response.status, response.read()
    except urllib.error.HTTPError as e:
        return e.code, b""


def test_health_endpoint_returns_ok() -> None:
    server = start_health_server(port=0)
    try:
        port = server.server_address[1]
        status, body = _get(port, "/health")
        assert status == 200
        assert body == b"ok"
    finally:
        server.shutdown()


def test_health_endpoint_accepts_trailing_slash() -> None:
    server = start_health_server(port=0)
    try:
        port = server.server_address[1]
        status, body = _get(port, "/health/")
        assert status == 200
        assert body == b"ok"
    finally:
        server.shutdown()


def test_unrelated_path_returns_404() -> None:
    server = start_health_server(port=0)
    try:
        port = server.server_address[1]
        status, _ = _get(port, "/nope")
        assert status == 404
    finally:
        server.shutdown()
