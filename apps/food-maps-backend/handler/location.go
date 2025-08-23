package handler

import (
	"context"
	"fmt"

	"github.com/tanjd/core-repository/apps/food-maps-backend/model"
)

type LocationService interface {
	CreateLocation(ctx context.Context, req *model.CreateLocationRequest) (*model.Location, error)
	GetLocation(ctx context.Context, id string) (*model.Location, error)
	UpdateLocation(ctx context.Context, id string, req *model.UpdateLocationRequest) (*model.Location, error)
	DeleteLocation(ctx context.Context, id string) error
	ListLocations(ctx context.Context, limit, offset int) ([]*model.Location, int, error)
}

type LocationHandler struct {
	service LocationService
}

func NewLocationHandler(service LocationService) *LocationHandler {
	return &LocationHandler{service: service}
}

// CreateLocation handles the creation of a new location
func (h *LocationHandler) CreateLocation(ctx context.Context, req *model.CreateLocationRequest) (*model.LocationResponse, error) {
	location, err := h.service.CreateLocation(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create location: %v", err)
	}

	return &model.LocationResponse{
		Body: *location,
	}, nil
}

// GetLocation retrieves a location by ID
func (h *LocationHandler) GetLocation(ctx context.Context, req *struct {
	ID string `path:"id" doc:"ID of the location to retrieve"`
}) (*model.LocationResponse, error) {
	location, err := h.service.GetLocation(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get location: %v", err)
	}

	if location == nil {
		return nil, fmt.Errorf("location not found")
	}

	return &model.LocationResponse{
		Body: *location,
	}, nil
}

// UpdateLocation updates an existing location
func (h *LocationHandler) UpdateLocation(ctx context.Context, req *model.UpdateLocationRequest) (*model.LocationResponse, error) {
	location, err := h.service.UpdateLocation(ctx, req.ID, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update location: %v", err)
	}

	if location == nil {
		return nil, fmt.Errorf("location not found")
	}

	return &model.LocationResponse{
		Body: *location,
	}, nil
}

// DeleteLocation removes a location
func (h *LocationHandler) DeleteLocation(ctx context.Context, req *struct {
	ID string `path:"id" doc:"ID of the location to delete"`
}) (*struct{}, error) {
	if err := h.service.DeleteLocation(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete location: %v", err)
	}

	return &struct{}{}, nil
}

// ListLocations retrieves a paginated list of locations
func (h *LocationHandler) ListLocations(ctx context.Context, req *struct {
	Limit  int `query:"limit" default:"20" doc:"Number of locations to return"`
	Offset int `query:"offset" default:"0" doc:"Number of locations to skip"`
}) (*model.LocationListResponse, error) {
	locations, total, err := h.service.ListLocations(ctx, req.Limit, req.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list locations: %v", err)
	}

	return &model.LocationListResponse{
		Body: struct {
			Locations []model.Location `json:"locations" doc:"List of locations"`
			Total     int              `json:"total" doc:"Total number of locations"`
		}{
			Locations: convertToLocationSlice(locations),
			Total:     total,
		},
	}, nil
}

func convertToLocationSlice(locations []*model.Location) []model.Location {
	result := make([]model.Location, len(locations))
	for i, loc := range locations {
		result[i] = *loc
	}
	return result
}
