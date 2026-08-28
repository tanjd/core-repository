CREATE TABLE recommendations (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id        INTEGER NOT NULL REFERENCES books(id),
    recommender_id INTEGER NOT NULL REFERENCES users(id),
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_recommendations_book_recommender ON recommendations(book_id, recommender_id);
