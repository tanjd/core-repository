import pytest

from my_python_project import main  # type: ignore


def test_main(capsys: pytest.CaptureFixture):  # type: ignore
    """Test the main function."""
    main()
    captured = capsys.readouterr()  # type: ignore
    assert captured.out == "Hello world\n"  # type: ignore
