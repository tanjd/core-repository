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

	// busy_timeout is set via the DSN (rather than a PRAGMA after open) so it
	// applies to the very first connection, before WAL mode is enabled below.
	db, err := gorm.Open(gormsqlite.Open(dbPath+"?_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// mattn/go-sqlite3 (cgo) serializes writes at the driver level, so capping
	// the pool at one connection avoids "database is locked" errors that a
	// larger pool would otherwise surface as write contention.
	sqlDB.SetMaxOpenConns(1)

	// WAL lets readers proceed without blocking on an in-progress writer
	// (SQLite's default rollback-journal mode blocks readers on writes).
	if _, err := sqlDB.ExecContext(context.Background(), "PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	if err := runMigrations(sqlDB); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}

	return db, nil
}

func runMigrations(sqlDB *sql.DB) error {
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

	log.Info().Msg("running database migrations")
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Error().Err(err).Msg("database migration failed")
		return err
	}

	log.Info().Msg("database schema up to date")
	return nil
}

// Seed inserts default app settings on first boot (idempotent).
func Seed(database *gorm.DB) {
	defaults := []models.AppSetting{
		{Key: "allow_registration", Value: "true"},
		{Key: "require_registration_approval", Value: "false"},
		{Key: "max_copies_per_user", Value: "10"},
		{Key: "require_verified_to_borrow", Value: "false"},
		{Key: "max_active_loans", Value: "0"},
		{Key: "cover_refresh_interval", Value: "24h"},
		{Key: "verification_requires_phone", Value: "false"},
		{Key: "verification_min_books_shared", Value: "0"},
		{Key: "require_email_confirmation_on_change", Value: "true"},
	}
	for _, s := range defaults {
		database.Where(models.AppSetting{Key: s.Key}).FirstOrCreate(&s)
	}
}
