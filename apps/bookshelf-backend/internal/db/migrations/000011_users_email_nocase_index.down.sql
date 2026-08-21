-- Dropping the index reinstates only the original BINARY-collation UNIQUE
-- constraint from 000001; a rolled-back binary's FindByEmail goes back to
-- case-sensitive matching, which is what it expects.
DROP INDEX IF EXISTS idx_users_email_nocase;
