"""Standard logging.basicConfig startup call shared across bots."""

import logging
import sys
from collections.abc import Sequence
from typing import TextIO

STANDARD_LOG_FORMAT = "%(asctime)s - %(name)s - %(levelname)s - %(message)s"


def configure_logging(
    *,
    level: int = logging.INFO,
    quiet_loggers: Sequence[str] = (),
    force: bool = False,
    stream: TextIO = sys.stdout,
) -> None:
    """Configure the root logger with the shared format, then quiet named loggers."""
    logging.basicConfig(format=STANDARD_LOG_FORMAT, level=level, stream=stream, force=force)
    for name in quiet_loggers:
        logging.getLogger(name).setLevel(logging.WARNING)
