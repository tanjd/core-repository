package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/tanjd/core-repository/apps/food-maps-backend/model"
)

type Router struct {
	LocationHandler LocationHandler
	api             huma.API
}

type LocationHandler interface {
	CreateLocation(context.Context, *model.CreateLocationRequest) (*model.LocationResponse, error)
	GetLocation(context.Context, *struct {
		ID string `path:"id" doc:"ID of the location to retrieve"`
	}) (*model.LocationResponse, error)
	UpdateLocation(context.Context, *model.UpdateLocationRequest) (*model.LocationResponse, error)
	DeleteLocation(context.Context, *struct {
		ID string `path:"id" doc:"ID of the location to delete"`
	}) (*struct{}, error)
	ListLocations(context.Context, *struct {
		Limit  int `query:"limit" default:"20" doc:"Number of locations to return"`
		Offset int `query:"offset" default:"0" doc:"Number of locations to skip"`
	}) (*model.LocationListResponse, error)
}

func NewRouter(locationHandler LocationHandler, api huma.API) *Router {
	return &Router{LocationHandler: locationHandler, api: api}
}

func (r *Router) AddLocationRoutes() {
	huma.Register(r.api, huma.Operation{
		OperationID: "create-location",
		Method:      http.MethodPost,
		Path:        "/locations",
		Summary:     "Create a new location",
		Tags:        []string{"Locations"},
	}, r.LocationHandler.CreateLocation)

	huma.Register(r.api, huma.Operation{
		OperationID: "get-location",
		Method:      http.MethodGet,
		Path:        "/locations/{id}",
		Summary:     "Get a location by ID",
		Tags:        []string{"Locations"},
	}, r.LocationHandler.GetLocation)

	huma.Register(r.api, huma.Operation{
		OperationID: "update-location",
		Method:      http.MethodPut,
		Path:        "/locations/{id}",
		Summary:     "Update a location",
		Tags:        []string{"Locations"},
	}, r.LocationHandler.UpdateLocation)

	huma.Register(r.api, huma.Operation{
		OperationID: "delete-location",
		Method:      http.MethodDelete,
		Path:        "/locations/{id}",
		Summary:     "Delete a location",
		Tags:        []string{"Locations"},
	}, r.LocationHandler.DeleteLocation)

	huma.Register(r.api, huma.Operation{
		OperationID: "list-locations",
		Method:      http.MethodGet,
		Path:        "/locations",
		Summary:     "List locations",
		Tags:        []string{"Locations"},
	}, r.LocationHandler.ListLocations)
}
