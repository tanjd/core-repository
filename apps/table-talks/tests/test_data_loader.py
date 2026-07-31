"""Tests for theme and question loading."""

from table_talks.data_loader import get_questions, get_themes


def test_get_themes_returns_list_with_ids():
    themes = get_themes()
    assert isinstance(themes, list)
    assert len(themes) >= 1
    for t in themes:
        assert "id" in t
        assert "label" in t
        assert isinstance(t["id"], str)
        assert isinstance(t["label"], str)


def test_get_questions_marriage_returns_strings():
    questions = get_questions("marriage")
    assert isinstance(questions, list)
    assert all(isinstance(q, str) and len(q) > 0 for q in questions)


def test_get_questions_unknown_theme_returns_empty():
    questions = get_questions("nonexistent-theme-id-xyz")
    assert questions == []


def test_get_questions_fun_light_and_faith_exist():
    assert len(get_questions("fun-light")) >= 1
    assert len(get_questions("faith")) >= 1
