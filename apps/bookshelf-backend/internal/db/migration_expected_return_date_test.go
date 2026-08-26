package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"
)

// newMigrateInstance opens a raw *sql.DB (no GORM, no WAL setup — the
// migration behavior under test doesn't need either) at dbPath and wraps it
// in a *migrate.Migrate driven by the same embedded migration source Open
// uses, so a test can stop partway through the migration chain to seed rows
// before the migration under test runs.
func newMigrateInstance(t *testing.T, dbPath string) (*sql.DB, *migrate.Migrate) {
	t.Helper()

	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}

	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		t.Fatalf("iofs.New: %v", err)
	}

	driver, err := sqlite3.WithInstance(sqlDB, &sqlite3.Config{})
	if err != nil {
		t.Fatalf("sqlite3.WithInstance: %v", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "sqlite3", driver)
	if err != nil {
		t.Fatalf("migrate.NewWithInstance: %v", err)
	}

	return sqlDB, m
}

// TestMigration014_BackfillsExpectedReturnDate seeds one loan request per
// status with a NULL expected_return_date at schema version 13, applies
// migration 14, and checks each status backfilled per the formula in
// apps/bookshelf/docs/return-date-default-spec.md's "Migration plan", plus
// the additive/dropped columns the same migration makes.
func TestMigration014_BackfillsExpectedReturnDate(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "backfill.db")
	sqlDB, m := newMigrateInstance(t, dbPath)
	defer sqlDB.Close()

	if err := m.Migrate(13); err != nil {
		t.Fatalf("migrate to v13: %v", err)
	}

	requestedAt, returnedAt, respondedAt := seedBackfillFixtures(t, sqlDB)

	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate to v14: %v", err)
	}

	assertBackfilledDates(t, sqlDB, requestedAt, returnedAt, respondedAt)
	assertNoNullReturnDates(t, sqlDB)
	assertAuditColumnsUnset(t, sqlDB, 1)
	assertColumnDropped(t, sqlDB, "copies", "return_date_required")
}

// seedBackfillFixtures inserts one user/book/copy plus one loan request per
// status (all with a NULL expected_return_date, except #6 which already has
// a real date and must survive the backfill untouched) at schema version 13,
// before migration 14's backfill+NOT NULL rebuild runs.
func seedBackfillFixtures(t *testing.T, sqlDB *sql.DB) (requestedAt, returnedAt, respondedAt string) {
	t.Helper()

	mustExec(t, sqlDB, `INSERT INTO users (id, name, email, password) VALUES (1, 'Owner', 'owner@example.com', 'x')`)
	mustExec(t, sqlDB, `INSERT INTO users (id, name, email, password) VALUES (2, 'Borrower', 'borrower@example.com', 'x')`)
	mustExec(t, sqlDB, `INSERT INTO books (id, title, author) VALUES (1, 'Some Book', 'Some Author')`)
	mustExec(t, sqlDB, `INSERT INTO copies (id, book_id, owner_id) VALUES (1, 1, 1)`)

	requestedAt = "2026-01-01 00:00:00"
	returnedAt = "2026-01-20 00:00:00"
	respondedAt = "2026-01-05 00:00:00"

	mustExec(t, sqlDB, `
		INSERT INTO loan_requests (id, copy_id, borrower_id, status, requested_at, expected_return_date)
		VALUES (1, 1, 2, 'pending', ?, NULL)`, requestedAt)
	mustExec(t, sqlDB, `
		INSERT INTO loan_requests (id, copy_id, borrower_id, status, requested_at, responded_at, expected_return_date)
		VALUES (2, 1, 2, 'accepted', ?, ?, NULL)`, requestedAt, respondedAt)
	mustExec(t, sqlDB, `
		INSERT INTO loan_requests (id, copy_id, borrower_id, status, requested_at, returned_at, expected_return_date)
		VALUES (3, 1, 2, 'returned', ?, ?, NULL)`, requestedAt, returnedAt)
	mustExec(t, sqlDB, `
		INSERT INTO loan_requests (id, copy_id, borrower_id, status, requested_at, expected_return_date)
		VALUES (4, 1, 2, 'rejected', ?, NULL)`, requestedAt)
	mustExec(t, sqlDB, `
		INSERT INTO loan_requests (id, copy_id, borrower_id, status, requested_at, expected_return_date)
		VALUES (5, 1, 2, 'cancelled', ?, NULL)`, requestedAt)
	// A loan that already had a real date must not be overwritten.
	mustExec(t, sqlDB, `
		INSERT INTO loan_requests (id, copy_id, borrower_id, status, requested_at, expected_return_date)
		VALUES (6, 1, 2, 'pending', ?, '2099-06-15 00:00:00')`, requestedAt)

	return requestedAt, returnedAt, respondedAt
}

// assertBackfilledDates checks each seeded loan request's expected_return_date
// against the per-status formula in
// apps/bookshelf/docs/return-date-default-spec.md's "Migration plan".
func assertBackfilledDates(t *testing.T, sqlDB *sql.DB, requestedAt, returnedAt, respondedAt string) {
	t.Helper()

	plus30 := func(base string) time.Time {
		t.Helper()
		return mustParse(t, base).AddDate(0, 0, 30)
	}

	cases := []struct {
		id   int
		want time.Time
	}{
		{1, plus30(requestedAt)},                 // pending -> requested_at + 30d
		{2, plus30(respondedAt)},                 // accepted -> responded_at + 30d
		{3, mustParse(t, returnedAt)},            // returned -> returned_at
		{4, plus30(requestedAt)},                 // rejected -> requested_at + 30d
		{5, plus30(requestedAt)},                 // cancelled -> requested_at + 30d
		{6, mustParse(t, "2099-06-15 00:00:00")}, // already had a date -> untouched
	}

	for _, tc := range cases {
		var got time.Time
		if err := sqlDB.QueryRowContext(context.Background(), `SELECT expected_return_date FROM loan_requests WHERE id = ?`, tc.id).Scan(&got); err != nil {
			t.Fatalf("id %d: query: %v", tc.id, err)
		}
		if !got.Equal(tc.want) {
			t.Errorf("id %d: expected_return_date = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func assertNoNullReturnDates(t *testing.T, sqlDB *sql.DB) {
	t.Helper()

	var nullCount int
	if err := sqlDB.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM loan_requests WHERE expected_return_date IS NULL`).Scan(&nullCount); err != nil {
		t.Fatalf("count nulls: %v", err)
	}
	if nullCount != 0 {
		t.Errorf("expected_return_date IS NULL count = %d, want 0", nullCount)
	}
}

func assertAuditColumnsUnset(t *testing.T, sqlDB *sql.DB, loanRequestID int) {
	t.Helper()

	var changedBy sql.NullInt64
	var changedAt sql.NullString
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT expected_return_date_changed_by, expected_return_date_changed_at FROM loan_requests WHERE id = ?`,
		loanRequestID,
	).Scan(&changedBy, &changedAt); err != nil {
		t.Fatalf("query audit columns: %v", err)
	}
	if changedBy.Valid || changedAt.Valid {
		t.Errorf("expected audit columns to be NULL for an unamended row, got changed_by=%v changed_at=%v", changedBy, changedAt)
	}
}

func assertColumnDropped(t *testing.T, sqlDB *sql.DB, table, column string) {
	t.Helper()

	var columnCount int
	if err := sqlDB.QueryRowContext(
		context.Background(),
		`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`,
		table, column,
	).Scan(&columnCount); err != nil {
		t.Fatalf("check %s columns: %v", table, err)
	}
	if columnCount != 0 {
		t.Errorf("expected %s.%s to be dropped, but the column still exists", table, column)
	}
}

func mustExec(t *testing.T, sqlDB *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := sqlDB.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return parsed
}
