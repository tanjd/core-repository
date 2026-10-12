CREATE TABLE to_read_entries (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id    INTEGER NOT NULL REFERENCES books(id),
    user_id    INTEGER NOT NULL REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(book_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_to_read_entries_book_id ON to_read_entries(book_id);
CREATE INDEX IF NOT EXISTS idx_to_read_entries_user_id ON to_read_entries(user_id);
