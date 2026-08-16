// Package db initializes and returns the application database handle.
package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/rs/zerolog/log"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

//go:embed migrations
var migrationsFS embed.FS

// Open initialises a SQLite database at the given path, creates the data
// directory if needed, runs versioned migrations, and returns the *gorm.DB handle.
func Open(dbPath string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return nil, err
	}

	db, err := gorm.Open(gormsqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	if err := runMigrations(sqlDB); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}

	return db, nil
}

func runMigrations(sqlDB *sql.DB) error {
	if err := ensureBaseline(sqlDB); err != nil {
		return err
	}

	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return err
	}

	driver, err := sqlite3.WithInstance(sqlDB, &sqlite3.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", src, "sqlite3", driver)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	log.Info().Msg("database schema up to date")
	return nil
}

// ensureBaseline sets the migration version to 7 for databases that were
// created by GORM AutoMigrate (all tables exist but no schema_migrations table).
// Fresh databases have no tables, so this is a no-op and migrations run normally.
func ensureBaseline(sqlDB *sql.DB) error {
	ctx := context.Background()
	var hasMigrationsTable int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`,
	).Scan(&hasMigrationsTable); err != nil {
		return fmt.Errorf("check schema_migrations table: %w", err)
	}
	if hasMigrationsTable > 0 {
		return nil // already tracking versions
	}

	var hasUsersTable int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='users'`,
	).Scan(&hasUsersTable); err != nil {
		return fmt.Errorf("check users table: %w", err)
	}
	if hasUsersTable == 0 {
		return nil // fresh install — let migrations run from 000001
	}

	// Existing DB from AutoMigrate: mark all migrations (000001-000007) as applied.
	_, err := sqlDB.ExecContext(ctx,
		`CREATE TABLE schema_migrations (version bigint NOT NULL, dirty boolean NOT NULL);
         INSERT INTO schema_migrations (version, dirty) VALUES (7, false);`,
	)
	if err != nil {
		return fmt.Errorf("ensureBaseline: %w", err)
	}
	log.Info().Int("version", 7).Msg("migration baseline set for existing database")
	return nil
}

// Seed inserts default app settings on first boot (idempotent).
func Seed(database *gorm.DB) {
	defaults := []models.AppSetting{
		{Key: "allow_registration", Value: "true"},
		{Key: "max_copies_per_user", Value: "10"},
		{Key: "require_verified_to_borrow", Value: "false"},
		{Key: "max_active_loans", Value: "0"},
		{Key: "cover_refresh_interval", Value: "24h"},
		{Key: "verification_requires_phone", Value: "false"},
		{Key: "verification_min_books_shared", Value: "0"},
	}
	for _, s := range defaults {
		database.Where(models.AppSetting{Key: s.Key}).FirstOrCreate(&s)
	}
}
