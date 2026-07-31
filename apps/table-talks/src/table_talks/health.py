"""Minimal HTTP server for health checks. Runs in a daemon thread."""

import logging
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer

logger = logging.getLogger(__name__)

DEFAULT_HEALTH_PORT = 9999


class _HealthHandler(BaseHTTPRequestHandler):
    """Responds 200 OK only to GET /health."""

    def do_GET(self) -> None:  # noqa: N802
        if self.path == "/health" or self.path == "/health/":
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.end_headers()
            self.wfile.write(b"ok")
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format: str, *args: object) -> None:
        logger.debug("health %s", args[0] if args else "")


def start_health_server(port: int = DEFAULT_HEALTH_PORT) -> None:
    """Start a daemon thread serving GET /health on the given port."""
    server = HTTPServer(("0.0.0.0", port), _HealthHandler)

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
    logger.info("Health check server listening on port %s", port)
