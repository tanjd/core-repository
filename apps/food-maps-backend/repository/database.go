package repository

import (
	"context"

	"github.com/tanjd/core-repository/apps/food-maps-backend/model"
)

// Database defines the interface that any database implementation must satisfy
type Database interface {
	// Location operations
	CreateLocation(ctx context.Context, loc *model.Location) error
	GetLocation(ctx context.Context, id string) (*model.Location, error)
	UpdateLocation(ctx context.Context, loc *model.Location) error
	DeleteLocation(ctx context.Context, id string) error
	ListLocations(ctx context.Context, limit, offset int) ([]*model.Location, error)

	// City operations
	CreateCity(ctx context.Context, city *model.City) error
	GetCity(ctx context.Context, id int64) (*model.City, error)
	GetCityByName(ctx context.Context, name string, countryID int64) (*model.City, error)
	ListCities(ctx context.Context) ([]*model.City, error)

	// Country operations
	CreateCountry(ctx context.Context, country *model.Country) error
	GetCountry(ctx context.Context, id int64) (*model.Country, error)
	GetCountryByName(ctx context.Context, name string) (*model.Country, error)
	ListCountries(ctx context.Context) ([]*model.Country, error)

	// Tag operations
	CreateTag(ctx context.Context, tag *model.Tag) error
	GetTag(ctx context.Context, id int64) (*model.Tag, error)
	GetTagByName(ctx context.Context, name string) (*model.Tag, error)
	ListTags(ctx context.Context) ([]*model.Tag, error)

	// Location-Tag operations
	AddLocationTag(ctx context.Context, locationID string, tagID int64) error
	RemoveLocationTag(ctx context.Context, locationID string, tagID int64) error
	GetLocationTags(ctx context.Context, locationID string) ([]*model.Tag, error)

	// Transaction support
	BeginTx(ctx context.Context) (Transaction, error)
}

// Transaction represents a database transaction
type Transaction interface {
	Commit() error
	Rollback() error
	Database
}
