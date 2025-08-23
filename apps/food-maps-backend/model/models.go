package model

import (
	"time"

	"github.com/google/uuid"
)

// Location represents a food location
type Location struct {
	ID            uuid.UUID `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	Description   string    `json:"description" db:"description"`
	GoogleMapsURL string    `json:"google_maps_url" db:"google_maps_url"`
	CityID        int64     `json:"city_id" db:"city_id"`
	City          *City     `json:"city,omitempty" db:"-"`
	Tags          []Tag     `json:"tags,omitempty" db:"-"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// City represents a city
type City struct {
	ID        int64    `json:"id" db:"id"`
	Name      string   `json:"name" db:"name"`
	CountryID int64    `json:"country_id" db:"country_id"`
	Country   *Country `json:"country,omitempty" db:"-"`
}

// Country represents a country
type Country struct {
	ID   int64  `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

// Tag represents a location tag/category
type Tag struct {
	ID   int64  `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

// LocationTag represents the many-to-many relationship between locations and tags
type LocationTag struct {
	LocationID uuid.UUID `json:"location_id" db:"location_id"`
	TagID      int64     `json:"tag_id" db:"tag_id"`
}

// CreateLocationRequest represents the request to create a new location
type CreateLocationRequest struct {
	Body struct {
		Name          string   `json:"name" doc:"Name of the location" required:"true"`
		Description   string   `json:"description" doc:"Description of the location"`
		GoogleMapsURL string   `json:"google_maps_url" doc:"Google Maps URL for the location" required:"true"`
		City          string   `json:"city" doc:"City where the location is" required:"true"`
		Country       string   `json:"country" doc:"Country where the location is" required:"true"`
		Tags          []string `json:"tags" doc:"Tags/categories for the location"`
	}
}

// UpdateLocationRequest represents the request to update a location
type UpdateLocationRequest struct {
	ID   string `path:"id" doc:"ID of the location to update"`
	Body struct {
		Name          *string   `json:"name" doc:"Name of the location"`
		Description   *string   `json:"description" doc:"Description of the location"`
		GoogleMapsURL *string   `json:"google_maps_url" doc:"Google Maps URL for the location"`
		City          *string   `json:"city" doc:"City where the location is"`
		Country       *string   `json:"country" doc:"Country where the location is"`
		Tags          *[]string `json:"tags" doc:"Tags/categories for the location"`
	}
}

// LocationResponse represents the response for location operations
type LocationResponse struct {
	Body Location `json:"body" doc:"The location data"`
}

// LocationListResponse represents the response for listing locations
type LocationListResponse struct {
	Body struct {
		Locations []Location `json:"locations" doc:"List of locations"`
		Total     int        `json:"total" doc:"Total number of locations"`
	}
}
