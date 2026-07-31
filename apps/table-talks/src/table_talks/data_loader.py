"""Load themes and question cards from pluggable data sources.

Supports CSV files (default) and Google Sheets (when configured).
Data source is selected based on environment configuration.
"""

from .data_sources import Theme, get_data_source

# Initialize data source based on environment configuration
# (CSV by default, Google Sheets if ENABLE_GOOGLE_SHEETS=true)
_data_source = get_data_source()

# Re-export Theme for backward compatibility
__all__ = ["Theme", "get_themes", "get_questions", "get_all_questions"]


def get_themes() -> list[Theme]:
    """Return list of theme dicts: id, label, description.

    Data source is determined by environment configuration.
    """
    return _data_source.get_themes()


def get_questions(theme_id: str) -> list[str]:
    """Return list of questions for a theme. theme_id must be from get_themes().

    Data source is determined by environment configuration.
    """
    return _data_source.get_questions(theme_id)


def get_all_questions() -> list[tuple[str, str]]:
    """Return list of (theme_id, question) tuples for all questions.

    Used for random mix mode to show which theme each question is from.
    Data source is determined by environment configuration.
    """
    return _data_source.get_all_questions()
