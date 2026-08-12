"""Tests for the health-check server (from telegram_bot_shared)."""

import urllib.request

from telegram_bot_shared.health import start_health_server


def test_health_endpoint_returns_ok() -> None:
    server = start_health_server(port=0)
    try:
        port = server.server_address[1]
        with urllib.request.urlopen(f"http://127.0.0.1:{port}/health", timeout=5) as response:
            assert response.status == 200
            assert response.read() == b"ok"
    finally:
        server.shutdown()
