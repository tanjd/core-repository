DROP INDEX IF EXISTS idx_wishlist_requests_google_books_id;
DROP INDEX IF EXISTS idx_wishlist_requests_ol_key;
DROP INDEX IF EXISTS idx_wishlist_requests_requester_id;
DROP INDEX IF EXISTS idx_wishlist_requests_status;
DROP TABLE IF EXISTS wishlist_requests;
-- SQLite can't cheaply drop notifications.wishlist_request_id back out
-- (would need a full table rebuild); no other migration in this repo drops a
-- column either, so it's left in place here, consistent with there being no
-- established column-drop pattern to follow.
