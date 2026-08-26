-- Migration is one-way (see the spec's "Migration plan" section): the only
-- thing restored is the dropped copies.return_date_required column, back to
-- its original default. loan_requests.expected_return_date stays NOT NULL,
-- the backfilled values stay, and expected_return_date_changed_{by,at} stay
-- in place too — no established pattern in this repo drops a column on
-- rollback (see prior migrations' down files), and un-backfilling dates
-- would lose information about which loans had a real date before this
-- migration ran.
ALTER TABLE copies ADD COLUMN return_date_required INTEGER NOT NULL DEFAULT 0;
