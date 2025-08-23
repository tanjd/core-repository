package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/tanjd/core-repository/apps/food-maps-backend/model"
	"github.com/tanjd/core-repository/apps/food-maps-backend/repository"
	"github.com/tanjd/core-repository/apps/food-maps-backend/repository/sqlite"
)

type LocationService struct {
	db repository.Database
}

func NewLocationService(db repository.Database) *LocationService {
	return &LocationService{db: db}
}

func (s *LocationService) CreateLocation(ctx context.Context, req *model.CreateLocationRequest) (*model.Location, error) {
	// Start a transaction
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			log.Error().Err(err).Msg("Failed to rollback transaction")
		}
	}()

	// Add transaction to context
	ctx = context.WithValue(ctx, sqlite.TxKey, tx)

	// Get or create country
	country, err := tx.GetCountryByName(ctx, req.Body.Country)
	if err != nil {
		return nil, fmt.Errorf("failed to get country: %w", err)
	}
	if country == nil {
		country = &model.Country{Name: req.Body.Country}
		if err := tx.CreateCountry(ctx, country); err != nil {
			return nil, fmt.Errorf("failed to create country: %w", err)
		}
	}

	// Get or create city
	city, err := tx.GetCityByName(ctx, req.Body.City, country.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get city: %w", err)
	}
	if city == nil {
		city = &model.City{Name: req.Body.City, CountryID: country.ID}
		if err := tx.CreateCity(ctx, city); err != nil {
			return nil, fmt.Errorf("failed to create city: %w", err)
		}
	}

	// Create location
	location := &model.Location{
		ID:            uuid.New(),
		Name:          req.Body.Name,
		Description:   req.Body.Description,
		GoogleMapsURL: req.Body.GoogleMapsURL,
		CityID:        city.ID,
	}

	if err := tx.CreateLocation(ctx, location); err != nil {
		return nil, fmt.Errorf("failed to create location: %w", err)
	}

	// Create tags
	for _, tagName := range req.Body.Tags {
		tag, err := tx.GetTagByName(ctx, tagName)
		if err != nil {
			return nil, fmt.Errorf("failed to get tag: %w", err)
		}
		if tag == nil {
			tag = &model.Tag{Name: tagName}
			if err := tx.CreateTag(ctx, tag); err != nil {
				return nil, fmt.Errorf("failed to create tag: %w", err)
			}
		}
		if err := tx.AddLocationTag(ctx, location.ID.String(), tag.ID); err != nil {
			return nil, fmt.Errorf("failed to add location tag: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Load the complete location with relationships
	return s.GetLocation(ctx, location.ID.String())
}

func (s *LocationService) GetLocation(ctx context.Context, id string) (*model.Location, error) {
	location, err := s.db.GetLocation(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get location: %w", err)
	}
	if location == nil {
		return nil, nil
	}

	// Get city
	city, err := s.db.GetCity(ctx, location.CityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get city: %w", err)
	}
	location.City = city

	// Get country
	if city != nil {
		country, err := s.db.GetCountry(ctx, city.CountryID)
		if err != nil {
			return nil, fmt.Errorf("failed to get country: %w", err)
		}
		city.Country = country
	}

	// Get tags
	tags, err := s.db.GetLocationTags(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get location tags: %w", err)
	}
	location.Tags = convertToTagSlice(tags)

	return location, nil
}

func (s *LocationService) UpdateLocation(ctx context.Context, id string, req *model.UpdateLocationRequest) (*model.Location, error) {
	// Start a transaction
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			log.Error().Err(err).Msg("Failed to rollback transaction")
		}
	}()

	// Add transaction to context
	ctx = context.WithValue(ctx, sqlite.TxKey, tx)

	// Get existing location
	location, err := tx.GetLocation(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get location: %w", err)
	}
	if location == nil {
		return nil, nil
	}

	// Update fields if provided
	if req.Body.Name != nil {
		location.Name = *req.Body.Name
	}
	if req.Body.Description != nil {
		location.Description = *req.Body.Description
	}
	if req.Body.GoogleMapsURL != nil {
		location.GoogleMapsURL = *req.Body.GoogleMapsURL
	}

	// Update city and country if provided
	if req.Body.City != nil && req.Body.Country != nil {
		// Get or create country
		country, err := tx.GetCountryByName(ctx, *req.Body.Country)
		if err != nil {
			return nil, fmt.Errorf("failed to get country: %w", err)
		}
		if country == nil {
			country = &model.Country{Name: *req.Body.Country}
			if err := tx.CreateCountry(ctx, country); err != nil {
				return nil, fmt.Errorf("failed to create country: %w", err)
			}
		}

		// Get or create city
		city, err := tx.GetCityByName(ctx, *req.Body.City, country.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get city: %w", err)
		}
		if city == nil {
			city = &model.City{Name: *req.Body.City, CountryID: country.ID}
			if err := tx.CreateCity(ctx, city); err != nil {
				return nil, fmt.Errorf("failed to create city: %w", err)
			}
		}

		location.CityID = city.ID
	}

	// Update location
	if err := tx.UpdateLocation(ctx, location); err != nil {
		return nil, fmt.Errorf("failed to update location: %w", err)
	}

	// Update tags if provided
	if req.Body.Tags != nil {
		// Remove existing tags
		existingTags, err := tx.GetLocationTags(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing tags: %w", err)
		}
		for _, tag := range existingTags {
			if err := tx.RemoveLocationTag(ctx, id, tag.ID); err != nil {
				return nil, fmt.Errorf("failed to remove tag: %w", err)
			}
		}

		// Add new tags
		for _, tagName := range *req.Body.Tags {
			tag, err := tx.GetTagByName(ctx, tagName)
			if err != nil {
				return nil, fmt.Errorf("failed to get tag: %w", err)
			}
			if tag == nil {
				tag = &model.Tag{Name: tagName}
				if err := tx.CreateTag(ctx, tag); err != nil {
					return nil, fmt.Errorf("failed to create tag: %w", err)
				}
			}
			if err := tx.AddLocationTag(ctx, id, tag.ID); err != nil {
				return nil, fmt.Errorf("failed to add tag: %w", err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Load the complete location with relationships
	return s.GetLocation(ctx, id)
}

func (s *LocationService) DeleteLocation(ctx context.Context, id string) error {
	return s.db.DeleteLocation(ctx, id)
}

func (s *LocationService) ListLocations(ctx context.Context, limit, offset int) ([]*model.Location, int, error) {
	locations, err := s.db.ListLocations(ctx, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list locations: %w", err)
	}

	// Load relationships for each location
	for _, location := range locations {
		// Get city
		city, err := s.db.GetCity(ctx, location.CityID)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get city: %w", err)
		}
		location.City = city

		// Get country
		if city != nil {
			country, err := s.db.GetCountry(ctx, city.CountryID)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to get country: %w", err)
			}
			city.Country = country
		}

		// Get tags
		tags, err := s.db.GetLocationTags(ctx, location.ID.String())
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get location tags: %w", err)
		}
		location.Tags = convertToTagSlice(tags)
	}

	// TODO: Implement proper count query in repository
	return locations, len(locations), nil
}

func convertToTagSlice(tags []*model.Tag) []model.Tag {
	result := make([]model.Tag, len(tags))
	for i, tag := range tags {
		result[i] = *tag
	}
	return result
}
