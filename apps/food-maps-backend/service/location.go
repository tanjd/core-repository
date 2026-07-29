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
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackTx(tx)

	// Add transaction to context
	ctx = context.WithValue(ctx, sqlite.TxKey, tx)

	city, err := getOrCreateCity(ctx, tx, req.Body.Country, req.Body.City)
	if err != nil {
		return nil, err
	}

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

	if err := addLocationTags(ctx, tx, location.ID.String(), req.Body.Tags); err != nil {
		return nil, err
	}

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
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackTx(tx)

	// Add transaction to context
	ctx = context.WithValue(ctx, sqlite.TxKey, tx)

	location, err := tx.GetLocation(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get location: %w", err)
	}
	if location == nil {
		return nil, nil
	}

	if req.Body.Name != nil {
		location.Name = *req.Body.Name
	}
	if req.Body.Description != nil {
		location.Description = *req.Body.Description
	}
	if req.Body.GoogleMapsURL != nil {
		location.GoogleMapsURL = *req.Body.GoogleMapsURL
	}

	if req.Body.City != nil && req.Body.Country != nil {
		city, err := getOrCreateCity(ctx, tx, *req.Body.Country, *req.Body.City)
		if err != nil {
			return nil, err
		}
		location.CityID = city.ID
	}

	if err := tx.UpdateLocation(ctx, location); err != nil {
		return nil, fmt.Errorf("failed to update location: %w", err)
	}

	if req.Body.Tags != nil {
		if err := replaceLocationTags(ctx, tx, id, *req.Body.Tags); err != nil {
			return nil, err
		}
	}

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

func rollbackTx(tx repository.Transaction) {
	if err := tx.Rollback(); err != nil {
		log.Error().Err(err).Msg("Failed to rollback transaction")
	}
}

func getOrCreateCountry(ctx context.Context, tx repository.Transaction, name string) (*model.Country, error) {
	country, err := tx.GetCountryByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get country: %w", err)
	}
	if country != nil {
		return country, nil
	}

	country = &model.Country{Name: name}
	if err := tx.CreateCountry(ctx, country); err != nil {
		return nil, fmt.Errorf("failed to create country: %w", err)
	}
	return country, nil
}

func getOrCreateCity(ctx context.Context, tx repository.Transaction, countryName, cityName string) (*model.City, error) {
	country, err := getOrCreateCountry(ctx, tx, countryName)
	if err != nil {
		return nil, err
	}

	city, err := tx.GetCityByName(ctx, cityName, country.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get city: %w", err)
	}
	if city != nil {
		return city, nil
	}

	city = &model.City{Name: cityName, CountryID: country.ID}
	if err := tx.CreateCity(ctx, city); err != nil {
		return nil, fmt.Errorf("failed to create city: %w", err)
	}
	return city, nil
}

func getOrCreateTag(ctx context.Context, tx repository.Transaction, name string) (*model.Tag, error) {
	tag, err := tx.GetTagByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag: %w", err)
	}
	if tag != nil {
		return tag, nil
	}

	tag = &model.Tag{Name: name}
	if err := tx.CreateTag(ctx, tag); err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}
	return tag, nil
}

func addLocationTags(ctx context.Context, tx repository.Transaction, locationID string, tagNames []string) error {
	for _, tagName := range tagNames {
		tag, err := getOrCreateTag(ctx, tx, tagName)
		if err != nil {
			return err
		}
		if err := tx.AddLocationTag(ctx, locationID, tag.ID); err != nil {
			return fmt.Errorf("failed to add location tag: %w", err)
		}
	}
	return nil
}

func replaceLocationTags(ctx context.Context, tx repository.Transaction, locationID string, tagNames []string) error {
	existingTags, err := tx.GetLocationTags(ctx, locationID)
	if err != nil {
		return fmt.Errorf("failed to get existing tags: %w", err)
	}
	for _, tag := range existingTags {
		if err := tx.RemoveLocationTag(ctx, locationID, tag.ID); err != nil {
			return fmt.Errorf("failed to remove tag: %w", err)
		}
	}

	return addLocationTags(ctx, tx, locationID, tagNames)
}
