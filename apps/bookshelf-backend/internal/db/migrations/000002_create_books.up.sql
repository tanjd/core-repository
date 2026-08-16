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
    google_books_id TEXT
);
