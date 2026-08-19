ALTER TABLE users ADD COLUMN reset_password_otp_code TEXT;
ALTER TABLE users ADD COLUMN reset_password_otp_expiry DATETIME;
