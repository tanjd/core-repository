"""Data source abstraction for themes and questions.

Supports multiple backends: CSV files, Google Sheets, etc.
"""

from .base import DataSource, Theme
from .factory import get_data_source

__all__ = ["DataSource", "Theme", "get_data_source"]
