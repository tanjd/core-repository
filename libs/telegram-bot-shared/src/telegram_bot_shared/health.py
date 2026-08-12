"""Minimal health-check HTTP endpoint, for container liveness probes.

Runs GET /health -> 200 "ok" on a background thread using only the
standard library, so it adds no extra dependency to a consuming bot.
"""

import logging
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer

logger = logging.getLogger(__name__)

DEFAULT_HEALTH_PORT = 8080


class _HealthHandler(BaseHTTPRequestHandler):
    """Responds 200 to GET /health (or /health/), 404 otherwise."""

    def do_GET(self) -> None:  # noqa: N802 (http.server API)
        if self.path == "/health" or self.path == "/health/":
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.end_headers()
            self.wfile.write(b"ok")
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format: str, *args: object) -> None:  # noqa: A002
        logger.debug("health %s", args[0] if args else "")


def start_health_server(port: int = DEFAULT_HEALTH_PORT) -> HTTPServer:
    """Start the health-check server on a daemon thread and return it."""
    server = HTTPServer(("0.0.0.0", port), _HealthHandler)  # noqa: S104

    def serve() -> None:
        try:
            server.serve_forever()
        except Exception as e:
            logger.warning("Health server stopped: %s", e)
        finally:
            try:
                server.server_close()
            except Exception:
                pass

    thread = threading.Thread(target=serve, daemon=True)
    thread.start()
    logger.info("Health check server listening on port %d", port)
    return server
