package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"github.com/tanjd/core-repository/apps/food-maps-backend/model"
	"github.com/tanjd/core-repository/apps/food-maps-backend/repository"
)

// TxKey is the context key for storing the transaction
type txKeyType struct{}

var TxKey = txKeyType{}

type SQLiteDB struct {
	db *sql.DB
}

// NewSQLiteDB creates a new SQLite database connection
func NewSQLiteDB(dbPath string) (*SQLiteDB, error) {
	log.Debug().Str("path", dbPath).Msg("Opening SQLite database")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}
	log.Debug().Msg("Successfully connected to database")

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("error enabling foreign keys: %w", err)
	}

	return &SQLiteDB{db: db}, nil
}

// Close closes the database connection
func (s *SQLiteDB) Close() error {
	log.Debug().Msg("Closing database connection")
	if s.db == nil {
		log.Warn().Msg("Database connection is already nil")
		return nil
	}
	return s.db.Close()
}

// executor returns the appropriate database executor (transaction or database connection)
// This is used by other functions in the package to handle database operations
// nolint:unused
func (s *SQLiteDB) executor(ctx context.Context) interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
} {
	if s.db == nil {
		log.Error().Msg("Database connection is nil")
		return nil
	}

	// Check if we have a transaction in the context
	if tx, ok := ctx.Value(TxKey).(*sql.Tx); ok && tx != nil {
		log.Debug().Msg("Using transaction from context")
		return tx
	}

	// Check if database is still open
	if err := s.db.Ping(); err != nil {
		log.Error().Err(err).Msg("Database connection is not alive")
		return nil
	}

	log.Debug().Msg("Using database connection")
	return s.db
}

// SQLiteTx wraps a sql.Tx to implement the Transaction interface
type SQLiteTx struct {
	*SQLiteDB
	tx *sql.Tx
}

// Commit commits the transaction
func (t *SQLiteTx) Commit() error {
	if t.tx == nil {
		return fmt.Errorf("transaction is nil")
	}
	return t.tx.Commit()
}

// Rollback rolls back the transaction
func (t *SQLiteTx) Rollback() error {
	if t.tx == nil {
		return nil // Already committed or rolled back
	}
	return t.tx.Rollback()
}

// BeginTx starts a new transaction
func (s *SQLiteDB) BeginTx(ctx context.Context) (repository.Transaction, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("error beginning transaction: %w", err)
	}

	return &SQLiteTx{
		SQLiteDB: s,
		tx:       tx,
	}, nil
}

// UpdateLocation updates a location in the database
func (s *SQLiteDB) UpdateLocation(ctx context.Context, loc *model.Location) error {
	exec := s.executor(ctx)
	if exec == nil {
		return fmt.Errorf("database connection is nil")
	}

	query := `UPDATE locations SET name = ?, description = ?, google_maps_url = ?, city_id = ? WHERE id = ?`
	_, err := exec.ExecContext(ctx, query, loc.Name, loc.Description, loc.GoogleMapsURL, loc.CityID, loc.ID.String())
	if err != nil {
		return fmt.Errorf("error updating location: %w", err)
	}

	return nil
}

// RemoveLocationTag removes a tag from a location
func (s *SQLiteDB) RemoveLocationTag(ctx context.Context, locationID string, tagID int64) error {
	exec := s.executor(ctx)
	if exec == nil {
		return fmt.Errorf("database connection is nil")
	}

	query := `DELETE FROM location_tags WHERE location_id = ? AND tag_id = ?`
	_, err := exec.ExecContext(ctx, query, locationID, tagID)
	if err != nil {
		return fmt.Errorf("error removing location tag: %w", err)
	}

	return nil
}

// ListTags retrieves all tags from the database
func (s *SQLiteDB) ListTags(ctx context.Context) ([]*model.Tag, error) {
	exec := s.executor(ctx)
	if exec == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `SELECT id, name FROM tags`
	rows, err := exec.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying tags: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing rows")
		}
	}()

	var tags []*model.Tag
	for rows.Next() {
		var tag model.Tag
		if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
			return nil, fmt.Errorf("error scanning tag: %w", err)
		}
		tags = append(tags, &tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return tags, nil
}

// ListLocations retrieves locations from the database with pagination
func (s *SQLiteDB) ListLocations(ctx context.Context, limit, offset int) ([]*model.Location, error) {
	exec := s.executor(ctx)
	if exec == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `SELECT id, name, description, google_maps_url, city_id FROM locations LIMIT ? OFFSET ?`
	rows, err := exec.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying locations: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing rows")
		}
	}()

	var locations []*model.Location
	for rows.Next() {
		var location model.Location
		var idStr string
		if err := rows.Scan(&idStr, &location.Name, &location.Description, &location.GoogleMapsURL, &location.CityID); err != nil {
			return nil, fmt.Errorf("error scanning location: %w", err)
		}

		// Parse UUID
		locationID, err := uuid.Parse(idStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing location ID: %w", err)
		}
		location.ID = locationID

		locations = append(locations, &location)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return locations, nil
}

// ListCountries retrieves all countries from the database
func (s *SQLiteDB) ListCountries(ctx context.Context) ([]*model.Country, error) {
	exec := s.executor(ctx)
	if exec == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `SELECT id, name FROM countries`
	rows, err := exec.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying countries: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing rows")
		}
	}()

	var countries []*model.Country
	for rows.Next() {
		var country model.Country
		if err := rows.Scan(&country.ID, &country.Name); err != nil {
			return nil, fmt.Errorf("error scanning country: %w", err)
		}
		countries = append(countries, &country)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return countries, nil
}

// ListCities retrieves all cities from the database
func (s *SQLiteDB) ListCities(ctx context.Context) ([]*model.City, error) {
	exec := s.executor(ctx)
	if exec == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `SELECT id, name, country_id FROM cities`
	rows, err := exec.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying cities: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing rows")
		}
	}()

	var cities []*model.City
	for rows.Next() {
		var city model.City
		if err := rows.Scan(&city.ID, &city.Name, &city.CountryID); err != nil {
			return nil, fmt.Errorf("error scanning city: %w", err)
		}
		cities = append(cities, &city)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return cities, nil
}

// GetTagByName retrieves a tag from the database by name
func (s *SQLiteDB) GetTagByName(ctx context.Context, name string) (*model.Tag, error) {
	exec := s.executor(ctx)
	if exec == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `SELECT id, name FROM tags WHERE name = ?`
	row := exec.QueryRowContext(ctx, query, name)

	var tag model.Tag
	err := row.Scan(&tag.ID, &tag.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error scanning tag: %w", err)
	}

	return &tag, nil
}

// GetTag retrieves a tag from the database by ID
func (s *SQLiteDB) GetTag(ctx context.Context, id int64) (*model.Tag, error) {
	exec := s.executor(ctx)
	if exec == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `SELECT id, name FROM tags WHERE id = ?`
	row := exec.QueryRowContext(ctx, query, id)

	var tag model.Tag
	err := row.Scan(&tag.ID, &tag.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error scanning tag: %w", err)
	}

	return &tag, nil
}

// GetLocationTags retrieves all tags for a location
func (s *SQLiteDB) GetLocationTags(ctx context.Context, locationID string) ([]*model.Tag, error) {
	exec := s.executor(ctx)
	if exec == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `
		SELECT t.id, t.name
		FROM tags t
		JOIN location_tags lt ON lt.tag_id = t.id
		WHERE lt.location_id = ?
	`
	rows, err := exec.QueryContext(ctx, query, locationID)
	if err != nil {
		return nil, fmt.Errorf("error querying location tags: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing rows")
		}
	}()

	var tags []*model.Tag
	for rows.Next() {
		var tag model.Tag
		if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
			return nil, fmt.Errorf("error scanning tag: %w", err)
		}
		tags = append(tags, &tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return tags, nil
}

// GetLocation retrieves a location from the database by ID
func (s *SQLiteDB) GetLocation(ctx context.Context, id string) (*model.Location, error) {
	exec := s.executor(ctx)
	if exec == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `SELECT id, name, description, google_maps_url, city_id FROM locations WHERE id = ?`
	row := exec.QueryRowContext(ctx, query, id)

	var location model.Location
	var idStr string
	err := row.Scan(&idStr, &location.Name, &location.Description, &location.GoogleMapsURL, &location.CityID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error scanning location: %w", err)
	}

	// Parse UUID
	locationID, err := uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing location ID: %w", err)
	}
	location.ID = locationID

	return &location, nil
}

// GetCountryByName retrieves a country from the database by name
func (s *SQLiteDB) GetCountryByName(ctx context.Context, name string) (*model.Country, error) {
	exec := s.executor(ctx)
	if exec == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `SELECT id, name FROM countries WHERE name = ?`
	row := exec.QueryRowContext(ctx, query, name)

	var country model.Country
	err := row.Scan(&country.ID, &country.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error scanning country: %w", err)
	}

	return &country, nil
}

// GetCountry retrieves a country from the database by ID
func (s *SQLiteDB) GetCountry(ctx context.Context, id int64) (*model.Country, error) {
	exec := s.executor(ctx)
	if exec == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `SELECT id, name FROM countries WHERE id = ?`
	row := exec.QueryRowContext(ctx, query, id)

	var country model.Country
	err := row.Scan(&country.ID, &country.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error scanning country: %w", err)
	}

	return &country, nil
}

// GetCityByName retrieves a city from the database by name and country ID
func (s *SQLiteDB) GetCityByName(ctx context.Context, name string, countryID int64) (*model.City, error) {
	exec := s.executor(ctx)
	if exec == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `SELECT id, name, country_id FROM cities WHERE name = ? AND country_id = ?`
	row := exec.QueryRowContext(ctx, query, name, countryID)

	var city model.City
	err := row.Scan(&city.ID, &city.Name, &city.CountryID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error scanning city: %w", err)
	}

	return &city, nil
}

// GetCity retrieves a city from the database by ID
func (s *SQLiteDB) GetCity(ctx context.Context, id int64) (*model.City, error) {
	exec := s.executor(ctx)
	if exec == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `SELECT id, name, country_id FROM cities WHERE id = ?`
	row := exec.QueryRowContext(ctx, query, id)

	var city model.City
	err := row.Scan(&city.ID, &city.Name, &city.CountryID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error scanning city: %w", err)
	}

	return &city, nil
}

// DeleteLocation deletes a location from the database
func (s *SQLiteDB) DeleteLocation(ctx context.Context, id string) error {
	exec := s.executor(ctx)
	if exec == nil {
		return fmt.Errorf("database connection is nil")
	}

	// First delete all location tags
	query := `DELETE FROM location_tags WHERE location_id = ?`
	_, err := exec.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting location tags: %w", err)
	}

	// Then delete the location
	query = `DELETE FROM locations WHERE id = ?`
	_, err = exec.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting location: %w", err)
	}

	return nil
}

// CreateTag creates a new tag in the database
func (s *SQLiteDB) CreateTag(ctx context.Context, tag *model.Tag) error {
	exec := s.executor(ctx)
	if exec == nil {
		return fmt.Errorf("database connection is nil")
	}

	query := `INSERT INTO tags (name) VALUES (?)`
	result, err := exec.ExecContext(ctx, query, tag.Name)
	if err != nil {
		return fmt.Errorf("error creating tag: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("error getting last insert id: %w", err)
	}
	tag.ID = id

	return nil
}

// CreateLocation creates a new location in the database
func (s *SQLiteDB) CreateLocation(ctx context.Context, loc *model.Location) error {
	exec := s.executor(ctx)
	if exec == nil {
		return fmt.Errorf("database connection is nil")
	}

	query := `INSERT INTO locations (id, name, description, google_maps_url, city_id) VALUES (?, ?, ?, ?, ?)`
	_, err := exec.ExecContext(ctx, query, loc.ID.String(), loc.Name, loc.Description, loc.GoogleMapsURL, loc.CityID)
	if err != nil {
		return fmt.Errorf("error creating location: %w", err)
	}

	return nil
}

// CreateCountry creates a new country in the database
func (s *SQLiteDB) CreateCountry(ctx context.Context, country *model.Country) error {
	exec := s.executor(ctx)
	if exec == nil {
		return fmt.Errorf("database connection is nil")
	}

	query := `INSERT INTO countries (name) VALUES (?)`
	result, err := exec.ExecContext(ctx, query, country.Name)
	if err != nil {
		return fmt.Errorf("error creating country: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("error getting last insert id: %w", err)
	}
	country.ID = id

	return nil
}

// CreateCity creates a new city in the database
func (s *SQLiteDB) CreateCity(ctx context.Context, city *model.City) error {
	exec := s.executor(ctx)
	if exec == nil {
		return fmt.Errorf("database connection is nil")
	}

	query := `INSERT INTO cities (name, country_id) VALUES (?, ?)`
	result, err := exec.ExecContext(ctx, query, city.Name, city.CountryID)
	if err != nil {
		return fmt.Errorf("error creating city: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("error getting last insert id: %w", err)
	}
	city.ID = id

	return nil
}

// AddLocationTag adds a tag to a location
func (s *SQLiteDB) AddLocationTag(ctx context.Context, locationID string, tagID int64) error {
	exec := s.executor(ctx)
	if exec == nil {
		return fmt.Errorf("database connection is nil")
	}

	query := `INSERT INTO location_tags (location_id, tag_id) VALUES (?, ?)`
	_, err := exec.ExecContext(ctx, query, locationID, tagID)
	if err != nil {
		return fmt.Errorf("error adding location tag: %w", err)
	}

	return nil
}
