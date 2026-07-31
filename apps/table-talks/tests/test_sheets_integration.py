"""Tests for Google Sheets data source integration."""

import json
from pathlib import Path
from typing import Any
from unittest.mock import MagicMock, patch

import pytest

from table_talks.data_sources.sheets_source import GoogleSheetsDataSource


@pytest.fixture
def mock_service_account_file(tmp_path: Path) -> str:
    """Create a mock service account JSON file."""
    creds_file = tmp_path / "service-account.json"
    creds_data = {
        "type": "service_account",
        "project_id": "test-project",
        "private_key_id": "key123",
        "private_key": "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----\n",
        "client_email": "test@test-project.iam.gserviceaccount.com",
        "client_id": "123456789",
    }
    creds_file.write_text(json.dumps(creds_data))
    return str(creds_file)


@pytest.fixture
def mock_sheet_data() -> list[list[str]]:
    """Sample Google Sheet data in denormalized format."""
    return [
        ["theme_id", "theme_label", "theme_description", "question"],
        ["marriage", "Marriage", "Questions for couples", "What makes you feel loved?"],
        ["marriage", "Marriage", "Questions for couples", "What's your favorite memory?"],
        ["faith", "Faith", "Faith questions", "When did you feel most at peace?"],
        ["fun", "Fun & Light", "Lighthearted starters", "What's your hidden talent?"],
    ]


@pytest.fixture
def mock_gspread_client(mock_sheet_data: list[list[str]]) -> Any:
    """Mock gspread client that returns test data."""
    with patch("table_talks.data_sources.sheets_source.gspread.authorize") as mock_authorize:
        # Create mock worksheet
        mock_worksheet = MagicMock()
        mock_worksheet.get_all_values.return_value = mock_sheet_data

        # Create mock spreadsheet
        mock_spreadsheet = MagicMock()
        mock_spreadsheet.worksheet.return_value = mock_worksheet

        # Create mock client
        mock_client = MagicMock()
        mock_client.open_by_key.return_value = mock_spreadsheet

        mock_authorize.return_value = mock_client
        yield mock_authorize


class TestGoogleSheetsDataSource:
    """Test GoogleSheetsDataSource functionality."""

    def test_authentication_with_file(self, mock_gspread_client, mock_service_account_file):
        """Test authentication using service account file."""
        with patch("table_talks.data_sources.sheets_source.service_account"):
            source = GoogleSheetsDataSource(
                sheet_id="test_sheet_id",
                credentials_file=mock_service_account_file,
            )
            assert source._client is not None

    def test_authentication_with_json(self, mock_gspread_client):
        """Test authentication using service account JSON string."""
        creds_json = json.dumps(
            {
                "type": "service_account",
                "project_id": "test",
                "private_key": "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----\n",
                "client_email": "test@test.iam.gserviceaccount.com",
            }
        )
        with patch("table_talks.data_sources.sheets_source.service_account"):
            source = GoogleSheetsDataSource(
                sheet_id="test_sheet_id",
                credentials_json=creds_json,
            )
            assert source._client is not None

    def test_get_themes(self, mock_gspread_client, mock_service_account_file):
        """Test loading themes from Google Sheets."""
        with patch("table_talks.data_sources.sheets_source.service_account"):
            source = GoogleSheetsDataSource(
                sheet_id="test_sheet_id",
                credentials_file=mock_service_account_file,
            )
            themes = source.get_themes()

            # Should deduplicate themes
            assert len(themes) == 3
            assert themes[0]["id"] == "marriage"
            assert themes[0]["label"] == "Marriage"
            assert themes[0]["description"] == "Questions for couples"
            assert themes[1]["id"] == "faith"
            assert themes[2]["id"] == "fun"

    def test_get_questions_for_theme(self, mock_gspread_client, mock_service_account_file):
        """Test loading questions for a specific theme."""
        with patch("table_talks.data_sources.sheets_source.service_account"):
            source = GoogleSheetsDataSource(
                sheet_id="test_sheet_id",
                credentials_file=mock_service_account_file,
            )
            questions = source.get_questions("marriage")

            assert len(questions) == 2
            assert questions[0] == "What makes you feel loved?"
            assert questions[1] == "What's your favorite memory?"

    def test_get_questions_invalid_theme(self, mock_gspread_client, mock_service_account_file):
        """Test getting questions for non-existent theme returns empty list."""
        with patch("table_talks.data_sources.sheets_source.service_account"):
            source = GoogleSheetsDataSource(
                sheet_id="test_sheet_id",
                credentials_file=mock_service_account_file,
            )
            questions = source.get_questions("nonexistent")

            assert questions == []

    def test_get_all_questions(self, mock_gspread_client, mock_service_account_file):
        """Test loading all questions with theme associations."""
        with patch("table_talks.data_sources.sheets_source.service_account"):
            source = GoogleSheetsDataSource(
                sheet_id="test_sheet_id",
                credentials_file=mock_service_account_file,
            )
            all_questions = source.get_all_questions()

            assert len(all_questions) == 4
            assert all_questions[0] == ("marriage", "What makes you feel loved?")
            assert all_questions[1] == ("marriage", "What's your favorite memory?")
            assert all_questions[2] == ("faith", "When did you feel most at peace?")
            assert all_questions[3] == ("fun", "What's your hidden talent?")

    def test_cache_reduces_api_calls(self, mock_gspread_client, mock_service_account_file):
        """Test that cache prevents redundant API calls."""
        with patch("table_talks.data_sources.sheets_source.service_account"):
            source = GoogleSheetsDataSource(
                sheet_id="test_sheet_id",
                credentials_file=mock_service_account_file,
                cache_ttl=60,  # 1 minute cache
            )

            # First call - should hit API
            themes1 = source.get_themes()
            call_count_after_first = mock_gspread_client.return_value.open_by_key.call_count

            # Second call - should use cache
            themes2 = source.get_themes()
            call_count_after_second = mock_gspread_client.return_value.open_by_key.call_count

            assert themes1 == themes2
            assert call_count_after_first == call_count_after_second  # No additional call

    def test_fallback_to_csv_on_api_error(self, mock_service_account_file):
        """Test fallback to CSV when Google Sheets API fails."""
        with patch("table_talks.data_sources.sheets_source.gspread.authorize") as mock_authorize:
            # Make API call fail
            mock_authorize.return_value.open_by_key.side_effect = Exception("API Error")

            with patch("table_talks.data_sources.sheets_source.service_account"):
                source = GoogleSheetsDataSource(
                    sheet_id="test_sheet_id",
                    credentials_file=mock_service_account_file,
                )

                # Should fall back to CSV
                themes = source.get_themes()
                assert len(themes) >= 1  # Should get themes from CSV

    def test_fallback_to_csv_on_auth_error(self):
        """Test fallback to CSV when authentication fails."""
        with patch(
            "table_talks.data_sources.sheets_source.service_account.Credentials.from_service_account_file"
        ) as mock_creds:
            mock_creds.side_effect = Exception("Auth failed")

            source = GoogleSheetsDataSource(
                sheet_id="test_sheet_id",
                credentials_file="/nonexistent/path.json",
            )

            # Should fall back to CSV
            themes = source.get_themes()
            assert len(themes) >= 1  # Should get themes from CSV

    def test_invalid_sheet_structure(self, mock_service_account_file):
        """Test handling of sheet with missing required columns."""
        with patch("table_talks.data_sources.sheets_source.gspread.authorize") as mock_authorize:
            # Mock sheet with missing column
            mock_worksheet = MagicMock()
            mock_worksheet.get_all_values.return_value = [
                ["theme_id", "question"],  # Missing theme_label and theme_description
                ["marriage", "What makes you feel loved?"],
            ]

            mock_spreadsheet = MagicMock()
            mock_spreadsheet.worksheet.return_value = mock_worksheet

            mock_client = MagicMock()
            mock_client.open_by_key.return_value = mock_spreadsheet
            mock_authorize.return_value = mock_client

            with patch("table_talks.data_sources.sheets_source.service_account"):
                source = GoogleSheetsDataSource(
                    sheet_id="test_sheet_id",
                    credentials_file=mock_service_account_file,
                )

                # Should fall back to CSV
                themes = source.get_themes()
                assert len(themes) >= 1  # CSV fallback

    def test_empty_sheet_falls_back(self, mock_service_account_file):
        """Test handling of empty Google Sheet."""
        with patch("table_talks.data_sources.sheets_source.gspread.authorize") as mock_authorize:
            mock_worksheet = MagicMock()
            mock_worksheet.get_all_values.return_value = []

            mock_spreadsheet = MagicMock()
            mock_spreadsheet.worksheet.return_value = mock_worksheet

            mock_client = MagicMock()
            mock_client.open_by_key.return_value = mock_spreadsheet
            mock_authorize.return_value = mock_client

            with patch("table_talks.data_sources.sheets_source.service_account"):
                source = GoogleSheetsDataSource(
                    sheet_id="test_sheet_id",
                    credentials_file=mock_service_account_file,
                )

                themes = source.get_themes()
                assert len(themes) >= 1  # CSV fallback

    def test_skips_rows_with_missing_fields(self, mock_service_account_file):
        """Test that rows with missing required fields are skipped."""
        with patch("table_talks.data_sources.sheets_source.gspread.authorize") as mock_authorize:
            mock_worksheet = MagicMock()
            mock_worksheet.get_all_values.return_value = [
                ["theme_id", "theme_label", "theme_description", "question"],
                ["marriage", "Marriage", "Questions", "Valid question"],
                ["", "Theme2", "Description", "Missing theme_id"],  # Should skip
                ["theme3", "", "Description", "Missing theme_label"],  # Should skip
                ["theme4", "Theme4", "Description", ""],  # Should skip - no question
                ["faith", "Faith", "Faith questions", "Valid question 2"],
            ]

            mock_spreadsheet = MagicMock()
            mock_spreadsheet.worksheet.return_value = mock_worksheet

            mock_client = MagicMock()
            mock_client.open_by_key.return_value = mock_spreadsheet
            mock_authorize.return_value = mock_client

            with patch("table_talks.data_sources.sheets_source.service_account"):
                source = GoogleSheetsDataSource(
                    sheet_id="test_sheet_id",
                    credentials_file=mock_service_account_file,
                )

                themes = source.get_themes()
                all_questions = source.get_all_questions()

                # Should only have 2 valid themes
                assert len(themes) == 2
                assert themes[0]["id"] == "marriage"
                assert themes[1]["id"] == "faith"

                # Should only have 2 valid questions
                assert len(all_questions) == 2
