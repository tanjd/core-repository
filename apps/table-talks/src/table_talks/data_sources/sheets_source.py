"""Google Sheets data source implementation with CSV fallback."""

import json
import logging
import time

import gspread
from google.oauth2 import service_account

from .base import DataSource, Theme
from .csv_source import CSVDataSource

logger = logging.getLogger(__name__)

# Required columns in the Google Sheet
REQUIRED_COLUMNS = ["theme_id", "theme_label", "theme_description", "question"]

# Maximum length for theme_id to fit in Telegram callback_data
# Format: "theme:{id}" must be <= 64 bytes
# So theme_id must be <= 64 - len("theme:") = 58 bytes
MAX_THEME_ID_LENGTH = 58


class GoogleSheetsDataSource(DataSource):
    """Load themes and questions from Google Sheets with caching and CSV fallback.

    Features:
    - Service account authentication (file or JSON env var)
    - In-memory caching with configurable TTL
    - Validates sheet structure and data
    - Falls back to CSV on any error
    - Parses denormalized sheet format into themes and questions
    """

    def __init__(
        self,
        sheet_id: str,
        sheet_name: str = "Questions",
        credentials_file: str | None = None,
        credentials_json: str | None = None,
        cache_ttl: int = 300,
        csv_fallback: CSVDataSource | None = None,
    ):
        """Initialize Google Sheets data source.

        Args:
            sheet_id: Google Sheet ID (from URL)
            sheet_name: Name of the sheet/tab (default: "Questions")
            credentials_file: Path to service account JSON file
            credentials_json: Service account JSON as string
            cache_ttl: Cache time-to-live in seconds (default: 300 = 5 minutes)
            csv_fallback: CSV data source to use on errors (creates new if None)
        """
        self.sheet_id = sheet_id
        self.sheet_name = sheet_name
        self.cache_ttl = cache_ttl
        self.csv_fallback = csv_fallback or CSVDataSource()

        # Cache for parsed data (separate variables for cleaner typing)
        self._cached_themes: list[Theme] | None = None
        self._cached_questions_by_theme: dict[str, list[str]] | None = None
        self._cached_all_questions: list[tuple[str, str]] | None = None
        self._last_fetch_time: float | None = None

        # Initialize Google Sheets client
        self._client: gspread.Client | None = None
        try:
            self._client = self._authenticate(credentials_file, credentials_json)
            logger.info("Successfully authenticated with Google Sheets API")
        except Exception as e:
            logger.error(f"Failed to authenticate with Google Sheets: {e}")
            logger.info("Will use CSV fallback for all requests")

    def _authenticate(
        self, credentials_file: str | None, credentials_json: str | None
    ) -> gspread.Client:
        """Authenticate with Google Sheets API using service account.

        Args:
            credentials_file: Path to service account JSON file
            credentials_json: Service account JSON as string

        Returns:
            Authenticated gspread client

        Raises:
            Various exceptions if authentication fails
        """
        # Try JSON string first (for cloud platforms)
        if credentials_json:
            creds_dict = json.loads(credentials_json)
            credentials = service_account.Credentials.from_service_account_info(
                creds_dict,
                scopes=[
                    "https://www.googleapis.com/auth/spreadsheets.readonly",
                    "https://www.googleapis.com/auth/drive.readonly",
                ],
            )
            return gspread.authorize(credentials)

        # Try file path (for Docker/local)
        if credentials_file:
            credentials = service_account.Credentials.from_service_account_file(
                credentials_file,
                scopes=[
                    "https://www.googleapis.com/auth/spreadsheets.readonly",
                    "https://www.googleapis.com/auth/drive.readonly",
                ],
            )
            return gspread.authorize(credentials)

        raise ValueError("Either credentials_file or credentials_json must be provided")

    def _is_cache_valid(self) -> bool:
        """Check if cache is still valid based on TTL."""
        if self._last_fetch_time is None:
            return False
        elapsed = time.time() - self._last_fetch_time
        return elapsed < self.cache_ttl

    def _fetch_and_parse_sheet(self) -> None:
        """Fetch data from Google Sheets and parse into cache.

        Raises:
            Exception on any error (caller should handle with CSV fallback)
        """
        if self._client is None:
            raise RuntimeError("Google Sheets client not initialized")

        # Open spreadsheet and worksheet
        spreadsheet = self._client.open_by_key(self.sheet_id)
        worksheet = spreadsheet.worksheet(self.sheet_name)

        # Get all data
        rows = worksheet.get_all_values()
        if not rows:
            raise ValueError("Sheet is empty")

        # Validate structure
        header = rows[0]
        if not all(col in header for col in REQUIRED_COLUMNS):
            missing = [col for col in REQUIRED_COLUMNS if col not in header]
            raise ValueError(f"Sheet missing required columns: {missing}")

        # Parse data
        col_indices = {col: header.index(col) for col in REQUIRED_COLUMNS}
        themes_dict: dict[str, Theme] = {}
        questions_by_theme: dict[str, list[str]] = {}
        all_questions: list[tuple[str, str]] = []

        for i, row in enumerate(rows[1:], start=2):
            # Skip incomplete rows
            if len(row) <= max(col_indices.values()):
                continue

            theme_id = row[col_indices["theme_id"]].strip()
            theme_label = row[col_indices["theme_label"]].strip()
            theme_desc = row[col_indices["theme_description"]].strip()
            question = row[col_indices["question"]].strip()

            # Skip rows with missing required fields
            if not (theme_id and theme_label and question):
                logger.warning(f"Skipping row {i} with missing required fields")
                continue

            # Validate theme_id length and characters
            if len(theme_id.encode("utf-8")) > MAX_THEME_ID_LENGTH:
                logger.error(
                    f"Skipping row {i} - theme_id '{theme_id}' too long "
                    f"({len(theme_id.encode('utf-8'))} bytes, max {MAX_THEME_ID_LENGTH})"
                )
                continue

            # Check for invalid characters in theme_id (control characters)
            if any(c in theme_id for c in ["\n", "\r", "\t"]):
                logger.error(
                    f"Skipping row {i} - theme_id '{theme_id}' contains invalid characters"
                )
                continue

            # Deduplicate themes
            if theme_id not in themes_dict:
                themes_dict[theme_id] = Theme(
                    id=theme_id, label=theme_label, description=theme_desc
                )
                questions_by_theme[theme_id] = []

            # Add question
            questions_by_theme[theme_id].append(question)
            all_questions.append((theme_id, question))

        # Update cache
        self._cached_themes = list(themes_dict.values())
        self._cached_questions_by_theme = questions_by_theme
        self._cached_all_questions = all_questions
        self._last_fetch_time = time.time()

        logger.info(
            f"Loaded {len(themes_dict)} themes and {len(all_questions)} questions "
            "from Google Sheets"
        )

    def _ensure_cache_loaded(self) -> bool:
        """Ensure cache is loaded with data from Google Sheets.

        Returns:
            True if cache loaded successfully, False if fell back to CSV
        """
        # Check if cache is valid
        if self._is_cache_valid():
            return True

        # Try to fetch from Google Sheets
        try:
            self._fetch_and_parse_sheet()
            return True
        except Exception as e:
            logger.warning(f"Failed to fetch from Google Sheets: {e}, using CSV fallback")
            return False

    def get_themes(self) -> list[Theme]:
        """Return list of all available themes."""
        if self._ensure_cache_loaded():
            return self._cached_themes or []

        # Fallback to CSV
        return self.csv_fallback.get_themes()

    def get_questions(self, theme_id: str) -> list[str]:
        """Return list of questions for a specific theme."""
        if self._ensure_cache_loaded() and self._cached_questions_by_theme is not None:
            return self._cached_questions_by_theme.get(theme_id, [])

        # Fallback to CSV
        return self.csv_fallback.get_questions(theme_id)

    def get_all_questions(self) -> list[tuple[str, str]]:
        """Return all questions with their theme_id."""
        if self._ensure_cache_loaded() and self._cached_all_questions is not None:
            return self._cached_all_questions

        # Fallback to CSV
        return self.csv_fallback.get_all_questions()
