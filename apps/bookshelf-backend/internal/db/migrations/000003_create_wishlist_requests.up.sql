CREATE TABLE wishlist_requests (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    requester_id       INTEGER NOT NULL REFERENCES users(id),
    title              TEXT NOT NULL,
    author             TEXT NOT NULL,
    isbn               TEXT,
    ol_key             TEXT,
    google_books_id    TEXT,
    cover_url          TEXT,
    notes              TEXT,
    status             TEXT NOT NULL DEFAULT 'open',
    fulfilled_book_id  INTEGER REFERENCES books(id),
    fulfilled_at       DATETIME,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_wishlist_requests_status ON wishlist_requests(status);
CREATE INDEX IF NOT EXISTS idx_wishlist_requests_requester_id ON wishlist_requests(requester_id);
CREATE INDEX IF NOT EXISTS idx_wishlist_requests_ol_key ON wishlist_requests(ol_key);
CREATE INDEX IF NOT EXISTS idx_wishlist_requests_google_books_id ON wishlist_requests(google_books_id);

ALTER TABLE notifications ADD COLUMN wishlist_request_id INTEGER REFERENCES wishlist_requests(id);
