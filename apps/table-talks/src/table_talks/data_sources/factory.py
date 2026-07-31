"""Factory for creating data sources based on configuration."""

import logging
import os

from .base import DataSource
from .csv_source import CSVDataSource

logger = logging.getLogger(__name__)


def get_data_source() -> DataSource:
    """Create and return appropriate data source based on environment configuration.

    Checks ENABLE_GOOGLE_SHEETS environment variable:
    - If "true" and Google Sheets credentials configured: returns GoogleSheetsDataSource
      (with CSV fallback on errors)
    - Otherwise: returns CSVDataSource

    Environment variables used:
    - ENABLE_GOOGLE_SHEETS: Set to "true" to enable Google Sheets
    - GOOGLE_SHEET_ID: The Google Sheet ID (from URL)
    - GOOGLE_SHEET_NAME: Sheet/tab name (default: "Questions")
    - GOOGLE_SERVICE_ACCOUNT_FILE: Path to service account JSON file
    - GOOGLE_SERVICE_ACCOUNT_JSON: Service account JSON as string (alternative)
    - SHEETS_CACHE_TTL: Cache duration in seconds (default: 300)

    Returns:
        DataSource: Configured data source instance
    """
    # Check if Google Sheets is enabled
    enable_sheets = os.getenv("ENABLE_GOOGLE_SHEETS", "").lower() == "true"

    if enable_sheets:
        sheet_id = os.getenv("GOOGLE_SHEET_ID")
        creds_file = os.getenv("GOOGLE_SERVICE_ACCOUNT_FILE")
        creds_json = os.getenv("GOOGLE_SERVICE_ACCOUNT_JSON")

        # Check if we have the required configuration
        if sheet_id and (creds_file or creds_json):
            try:
                from .sheets_source import GoogleSheetsDataSource

                sheet_name = os.getenv("GOOGLE_SHEET_NAME", "Questions")
                cache_ttl = int(os.getenv("SHEETS_CACHE_TTL", "300"))

                logger.info("Attempting to load data from Google Sheets")
                return GoogleSheetsDataSource(
                    sheet_id=sheet_id,
                    sheet_name=sheet_name,
                    credentials_file=creds_file,
                    credentials_json=creds_json,
                    cache_ttl=cache_ttl,
                )
            except Exception as e:
                logger.warning(f"Failed to initialize Google Sheets: {e}")
                logger.info("Falling back to CSV")
        else:
            missing: list[str] = []
            if not sheet_id:
                missing.append("GOOGLE_SHEET_ID")
            if not (creds_file or creds_json):
                missing.append("GOOGLE_SERVICE_ACCOUNT_FILE or GOOGLE_SERVICE_ACCOUNT_JSON")
            logger.warning(f"Google Sheets enabled but missing config: {', '.join(missing)}")
            logger.info("Falling back to CSV")

    # Default to CSV
    logger.info("Loading data from CSV files")
    return CSVDataSource()
