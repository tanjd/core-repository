"""Abstract base class for data sources."""

from abc import ABC, abstractmethod
from typing import TypedDict


class Theme(TypedDict):
    """One theme with id, label, and description."""

    id: str
    label: str
    description: str


class DataSource(ABC):
    """Abstract interface for loading themes and questions."""

    @abstractmethod
    def get_themes(self) -> list[Theme]:
        """Return list of all available themes."""
        pass

    @abstractmethod
    def get_questions(self, theme_id: str) -> list[str]:
        """Return list of questions for a specific theme.

        Args:
            theme_id: The theme identifier (must exist in get_themes())

        Returns:
            List of question strings, or empty list if theme_id is invalid
        """
        pass

    @abstractmethod
    def get_all_questions(self) -> list[tuple[str, str]]:
        """Return all questions with their theme_id.

        Returns:
            List of (theme_id, question) tuples for all questions
        """
        pass
