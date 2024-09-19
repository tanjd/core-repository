from my_python_library import bye


def test_bye():
    """Test the bye function."""
    input = "my-python-library"
    expected = "Bye my-python-library"
    assert bye(input) == expected

    input = ""
    expected = "Bye nobody"
    assert bye(input) == expected
