ALTER TABLE users ADD COLUMN telegram_chat_id INTEGER;
ALTER TABLE users ADD COLUMN telegram_linked_at DATETIME;
ALTER TABLE users ADD COLUMN telegram_notifications_enabled BOOLEAN NOT NULL DEFAULT true;
CREATE UNIQUE INDEX idx_users_telegram_chat_id ON users(telegram_chat_id);
