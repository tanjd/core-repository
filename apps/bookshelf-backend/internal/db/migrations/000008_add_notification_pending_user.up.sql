ALTER TABLE notifications ADD COLUMN pending_user_id INTEGER REFERENCES users(id);
