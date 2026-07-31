"""Test validation of theme data and callback data."""

from table_talks.bot.keyboards import is_valid_callback_data


class TestCallbackDataValidation:
    """Test callback_data validation."""

    def test_valid_callback_data(self):
        """Test valid callback data passes validation."""
        assert is_valid_callback_data("theme:marriage")
        assert is_valid_callback_data("theme:fun-light")
        assert is_valid_callback_data("theme:faith")
        assert is_valid_callback_data("next")
        assert is_valid_callback_data("previous")

    def test_empty_callback_data(self):
        """Test empty callback data is invalid."""
        assert not is_valid_callback_data("")

    def test_too_long_callback_data(self):
        """Test callback data exceeding 64 bytes is invalid."""
        # Create a string that's exactly 65 bytes
        long_data = "theme:" + "x" * 59
        assert not is_valid_callback_data(long_data)

    def test_callback_data_with_newline(self):
        """Test callback data with newline is invalid."""
        assert not is_valid_callback_data("theme:test\nid")

    def test_callback_data_with_tab(self):
        """Test callback data with tab is invalid."""
        assert not is_valid_callback_data("theme:test\tid")

    def test_callback_data_with_carriage_return(self):
        """Test callback data with carriage return is invalid."""
        assert not is_valid_callback_data("theme:test\rid")

    def test_max_length_callback_data(self):
        """Test callback data at exactly 64 bytes is valid."""
        # Create a string that's exactly 64 bytes
        max_data = "theme:" + "x" * 58
        assert is_valid_callback_data(max_data)

    def test_unicode_callback_data(self):
        """Test callback data with unicode characters."""
        # Single emoji is typically 4 bytes in UTF-8
        assert is_valid_callback_data("theme:test🎲")

        # A moderate string of emojis should pass (4 bytes each)
        emoji_string = "theme:" + "🎲" * 14  # 62 bytes (6 + 56)
        assert is_valid_callback_data(emoji_string)

        # Too many emojis should fail
        emoji_string_long = "theme:" + "🎲" * 15  # 66 bytes (exceeds 64)
        assert not is_valid_callback_data(emoji_string_long)
