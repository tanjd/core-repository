ALTER TABLE users ADD COLUMN pending_email TEXT;
ALTER TABLE users ADD COLUMN pending_email_otp_code TEXT;
ALTER TABLE users ADD COLUMN pending_email_otp_expiry DATETIME;
