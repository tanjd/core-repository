ALTER TABLE users ADD COLUMN invited_by_id INTEGER REFERENCES users(id);
