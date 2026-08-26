package db

import (
	"path/filepath"
	"testing"
)

func TestMigrationVersion_afterOpen(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	gormDB, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("DB(): %v", err)
	}

	version, err := MigrationVersion(sqlDB)
	if err != nil {
		t.Fatalf("MigrationVersion: %v", err)
	}

	// Keep in sync with the highest numbered *.up.sql in internal/db/migrations/.
	const expectedMigrationCount uint = 13
	if version != expectedMigrationCount {
		t.Fatalf("MigrationVersion() = %d, want %d", version, expectedMigrationCount)
	}
}
