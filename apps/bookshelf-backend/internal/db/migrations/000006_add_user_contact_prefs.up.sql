ALTER TABLE users ADD COLUMN email_notifications_enabled BOOLEAN NOT NULL DEFAULT 1;
ALTER TABLE users ADD COLUMN telegram_username TEXT;
ALTER TABLE users ADD COLUMN whatsapp_username TEXT;
