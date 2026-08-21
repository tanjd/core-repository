-- Carries the Name/Email/Password/Phone typed on the registration form's
-- first step server-side, next to the OTP code they'll be verified with, so
-- clicking the emailed magic link on a different device finishes signup
-- without the browser that started it (see verifyRegisterEmailOTP →
-- finalizeRegistration in internal/handlers/auth.go). Nullable: the phone
-- channel's rows carry none of this, and neither do rows written before
-- this migration.
--
-- pending_email exists alongside the row's `identifier` because identifier
-- is normalized (lowercased) so the magic-link token can key off it, while
-- the account should be created with the casing the user actually typed —
-- that's what gets shown back to them. Lookups don't depend on it either
-- way: users.email is matched case-insensitively (see migration 000011).
ALTER TABLE registration_verifications ADD COLUMN pending_name TEXT;
ALTER TABLE registration_verifications ADD COLUMN pending_email TEXT;
ALTER TABLE registration_verifications ADD COLUMN pending_password_hash TEXT;
ALTER TABLE registration_verifications ADD COLUMN pending_phone TEXT;
