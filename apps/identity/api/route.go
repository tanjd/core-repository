package api

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/tanjd/core-repository/apps/identity/handler"
)

type Router struct {
	UserHandler *handler.UserHandler
	api         huma.API
}

func NewRouter(userHandler *handler.UserHandler, api huma.API) *Router {
	return &Router{UserHandler: userHandler, api: api}
}

func (r *Router) AddUserRoutes() {
	huma.Register(r.api, huma.Operation{
		OperationID:   "create-user",
		Method:        http.MethodPost,
		Path:          "/users",
		Summary:       "Create a new user",
		Tags:          []string{"Users"},
		DefaultStatus: http.StatusCreated,
	}, r.UserHandler.CreateUserHandler)

	huma.Register(r.api, huma.Operation{
		OperationID: "get-user",
		Method:      http.MethodGet,
		Path:        "/users/{id}",
		Summary:     "Get a user by ID",
		Tags:        []string{"Users"},
	}, r.UserHandler.GetUserHandler)
}

func (r *Router) AddHealthCheckRoutes() {
	huma.Register(r.api, huma.Operation{
		OperationID: "health-check",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Health check",
		Tags:        []string{"Health"},
	}, handler.HealthCheckHandler)
}
