from datetime import timedelta

from otobr_buddy.timeutil import utcnow


def test_upsert_user_then_get(db):
    db.upsert_user("1", "alice", "Alice")

    user = db.get_user("1")

    assert user is not None
    assert user["username"] == "alice"
    assert user["first_name"] == "Alice"


def test_upsert_user_updates_existing_row(db):
    db.upsert_user("1", "alice", "Alice")
    db.upsert_user("1", "alice2", "Alice")

    user = db.get_user("1")

    assert user["username"] == "alice2"


def test_create_and_end_partnership(db):
    db.upsert_user("1", "alice", "Alice")
    db.upsert_user("2", "bob", "Bob")

    partnership_id = db.create_partnership("1", "2")

    active_for_alice = db.get_active_partnerships_for_user("1")
    active_for_bob = db.get_active_partnerships_for_user("2")
    assert len(active_for_alice) == 1
    assert len(active_for_bob) == 1
    assert active_for_alice[0]["id"] == partnership_id

    db.end_partnership(partnership_id)

    assert db.get_active_partnerships_for_user("1") == []
    ended = db.get_ended_partnerships_for_user("1")
    assert len(ended) == 1
    assert ended[0]["id"] == partnership_id


def test_create_partnership_with_group_chat(db):
    db.upsert_user("1", "alice", "Alice")
    db.upsert_user("2", "bob", "Bob")

    partnership_id = db.create_partnership("1", "2", group_chat_id="-100123")

    partnership = db.get_partnership(partnership_id)
    assert partnership["group_chat_id"] == "-100123"
    assert db.get_active_partnership_for_group("-100123")["id"] == partnership_id


def test_get_active_partnership_for_group_ignores_ended(db):
    db.upsert_user("1", "alice", "Alice")
    db.upsert_user("2", "bob", "Bob")
    partnership_id = db.create_partnership("1", "2", group_chat_id="-100123")

    db.end_partnership(partnership_id)

    assert db.get_active_partnership_for_group("-100123") is None


def test_get_active_partnership_for_group_no_match(db):
    assert db.get_active_partnership_for_group("-100999") is None


def test_other_user_id(db):
    partnership = {"user_a_id": "1", "user_b_id": "2"}

    assert db.other_user_id(partnership, "1") == "2"
    assert db.other_user_id(partnership, "2") == "1"


def test_sessions_are_ordered_oldest_first(db):
    db.upsert_user("1", "alice", "Alice")
    db.upsert_user("2", "bob", "Bob")
    partnership_id = db.create_partnership("1", "2")

    db.add_session(partnership_id, "Romans 1", "1")
    db.add_session(partnership_id, "Romans 2", "2")

    sessions = db.get_sessions_for_partnership(partnership_id)

    assert [s["text_covered"] for s in sessions] == ["Romans 1", "Romans 2"]
    assert db.count_sessions_for_partnership(partnership_id) == 2
    assert db.get_last_session(partnership_id)["text_covered"] == "Romans 2"


def test_invite_lifecycle(db):
    db.upsert_user("1", "alice", "Alice")

    db.create_invite("ABC123", "1", utcnow() + timedelta(hours=1))
    invite = db.get_invite("ABC123")

    assert invite is not None
    assert invite["used_by"] is None

    db.mark_invite_used("ABC123", "2")
    invite = db.get_invite("ABC123")

    assert invite["used_by"] == "2"


def test_pending_pair_lifecycle(db):
    db.upsert_user("1", "alice", "Alice")

    assert db.get_pending_pair("-100123") is None

    db.create_pending_pair("-100123", "1", utcnow() + timedelta(hours=1))
    pending = db.get_pending_pair("-100123")

    assert pending is not None
    assert pending["claimant_id"] == "1"

    db.clear_pending_pair("-100123")

    assert db.get_pending_pair("-100123") is None


def test_pending_pair_overwritten_by_new_claim(db):
    db.upsert_user("1", "alice", "Alice")
    db.create_pending_pair("-100123", "1", utcnow() + timedelta(hours=1))

    # Same group chat, a different (e.g. re-)claim overwrites rather than erroring.
    db.create_pending_pair("-100123", "1", utcnow() + timedelta(hours=2))

    pending = db.get_pending_pair("-100123")
    assert pending["claimant_id"] == "1"


def test_frequency_setters(db):
    db.upsert_user("1", "alice", "Alice")
    db.upsert_user("2", "bob", "Bob")
    partnership_id = db.create_partnership("1", "2")

    db.set_frequency_interval(partnership_id, 7)
    partnership = db.get_partnership(partnership_id)
    assert partnership["frequency_mode"] == db.FREQUENCY_INTERVAL
    assert partnership["frequency_interval_days"] == 7

    db.set_frequency_weekly(partnership_id, 1, "19:00")
    partnership = db.get_partnership(partnership_id)
    assert partnership["frequency_mode"] == db.FREQUENCY_WEEKLY
    assert partnership["frequency_day_of_week"] == 1
    assert partnership["frequency_time"] == "19:00"
    # Switching modes clears the other mode's fields.
    assert partnership["frequency_interval_days"] is None
