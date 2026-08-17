CREATE INDEX IF NOT EXISTS idx_copies_book_id ON copies(book_id);
CREATE INDEX IF NOT EXISTS idx_copies_owner_id ON copies(owner_id);
CREATE INDEX IF NOT EXISTS idx_loan_requests_copy_id ON loan_requests(copy_id);
CREATE INDEX IF NOT EXISTS idx_loan_requests_borrower_id ON loan_requests(borrower_id);
CREATE INDEX IF NOT EXISTS idx_notifications_recipient_id ON notifications(recipient_id);
