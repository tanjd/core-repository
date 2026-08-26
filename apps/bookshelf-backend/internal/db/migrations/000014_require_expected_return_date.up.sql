-- Backfill any NULL expected_return_date before the NOT NULL constraint
-- below is added — see apps/bookshelf/docs/return-date-default-spec.md
-- "Migration plan" for the per-status rationale.
UPDATE loan_requests
SET expected_return_date = datetime(requested_at, '+30 days')
WHERE expected_return_date IS NULL AND status = 'pending';

UPDATE loan_requests
SET expected_return_date = datetime(COALESCE(loaned_at, responded_at, requested_at), '+30 days')
WHERE expected_return_date IS NULL AND status = 'accepted';

UPDATE loan_requests
SET expected_return_date = returned_at
WHERE expected_return_date IS NULL AND status = 'returned';

UPDATE loan_requests
SET expected_return_date = datetime(requested_at, '+30 days')
WHERE expected_return_date IS NULL AND status IN ('rejected', 'cancelled');

-- SQLite has no ALTER COLUMN ... SET NOT NULL, so making expected_return_date
-- required means the standard rebuild-and-swap: create the new shape, copy
-- every row across, drop the old table, rename the new one into place. This
-- also adds the two new audit columns in the same rebuild.
CREATE TABLE loan_requests_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    copy_id INTEGER NOT NULL REFERENCES copies(id),
    borrower_id INTEGER NOT NULL REFERENCES users(id),
    message TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    requested_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    responded_at DATETIME,
    loaned_at DATETIME,
    returned_at DATETIME,
    returned_by INTEGER REFERENCES users(id),
    expected_return_date DATETIME NOT NULL,
    expected_return_date_changed_by INTEGER REFERENCES users(id),
    expected_return_date_changed_at DATETIME
);

INSERT INTO loan_requests_new (
    id, copy_id, borrower_id, message, status, requested_at, responded_at,
    loaned_at, returned_at, returned_by, expected_return_date
)
SELECT
    id, copy_id, borrower_id, message, status, requested_at, responded_at,
    loaned_at, returned_at, returned_by, expected_return_date
FROM loan_requests;

DROP TABLE loan_requests;
ALTER TABLE loan_requests_new RENAME TO loan_requests;

CREATE INDEX IF NOT EXISTS idx_loan_requests_copy_id ON loan_requests(copy_id);
CREATE INDEX IF NOT EXISTS idx_loan_requests_borrower_id ON loan_requests(borrower_id);

-- Every Copy now behaves as if return_date_required were always true.
ALTER TABLE copies DROP COLUMN return_date_required;
