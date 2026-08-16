CREATE TABLE copies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id INTEGER NOT NULL REFERENCES books(id),
    owner_id INTEGER NOT NULL REFERENCES users(id),
    condition TEXT,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'available',
    auto_approve INTEGER NOT NULL DEFAULT 0,
    return_date_required INTEGER NOT NULL DEFAULT 0
);
