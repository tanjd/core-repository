CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    phone TEXT,
    password TEXT NOT NULL,
    verified INTEGER NOT NULL DEFAULT 0,
    role TEXT NOT NULL DEFAULT 'user',
    otp_code TEXT,
    otp_expiry DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    google_books_api_key TEXT,
    suspended BOOLEAN NOT NULL DEFAULT FALSE,
    pending_email TEXT,
    pending_email_otp_code TEXT,
    pending_email_otp_expiry DATETIME,
    pending_approval BOOLEAN NOT NULL DEFAULT FALSE,
    phone_verified BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE books (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    author TEXT NOT NULL,
    isbn TEXT,
    ol_key TEXT,
    cover_url TEXT,
    description TEXT,
    publisher TEXT,
    published_date TEXT,
    page_count INTEGER,
    language TEXT,
    google_books_id TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE copies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id INTEGER NOT NULL REFERENCES books(id),
    owner_id INTEGER NOT NULL REFERENCES users(id),
    condition TEXT,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'available',
    auto_approve INTEGER NOT NULL DEFAULT 0,
    return_date_required INTEGER NOT NULL DEFAULT 0,
    hide_owner INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE loan_requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    copy_id INTEGER NOT NULL REFERENCES copies(id),
    borrower_id INTEGER NOT NULL REFERENCES users(id),
    message TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    requested_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    responded_at DATETIME,
    loaned_at DATETIME,
    returned_at DATETIME,
    expected_return_date DATETIME
);

CREATE TABLE notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    recipient_id INTEGER NOT NULL REFERENCES users(id),
    type TEXT NOT NULL,
    loan_request_id INTEGER REFERENCES loan_requests(id),
    read INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS waitlist_entries (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    copy_id    INTEGER NOT NULL REFERENCES copies(id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
    UNIQUE (copy_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_copies_book_id ON copies(book_id);
CREATE INDEX IF NOT EXISTS idx_copies_owner_id ON copies(owner_id);
CREATE INDEX IF NOT EXISTS idx_loan_requests_copy_id ON loan_requests(copy_id);
CREATE INDEX IF NOT EXISTS idx_loan_requests_borrower_id ON loan_requests(borrower_id);
CREATE INDEX IF NOT EXISTS idx_notifications_recipient_id ON notifications(recipient_id);

CREATE TABLE registration_verifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel TEXT NOT NULL,
    identifier TEXT NOT NULL,
    code TEXT NOT NULL,
    expires_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX idx_registration_verifications_channel_identifier
    ON registration_verifications (channel, identifier);
