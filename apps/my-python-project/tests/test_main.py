from my_python_project import main


def test_main(capsys):
    """Test the main function."""
    main()
    captured = capsys.readouterr()
    assert captured.out == "Hello world\n"
