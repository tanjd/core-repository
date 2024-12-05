package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/tanjd/core-repository/apps/identity/handler"
	"github.com/tanjd/core-repository/apps/identity/model"
)

type Router struct {
	UserHandler           UserHandler
	AuthenticationHandler AuthenticationHandler
	api                   huma.API
}

type UserHandler interface {
	CreateUser(context.Context, *model.CreateUserRequest) (*model.CreateUserResponse, error)
	GetUser(context.Context, *model.GetUserRequest) (*model.GetUserResponse, error)
}

type AuthenticationHandler interface {
	RegisterUser(context.Context, *model.RegisterUserRequest) (*model.RegisterUserResponse, error)
	LoginUser(context.Context, *model.LoginUserRequest) (*model.LoginUserResponse, error)
}

func NewRouter(userHandler UserHandler, authenticationHandler AuthenticationHandler, api huma.API) *Router {
	return &Router{UserHandler: userHandler, AuthenticationHandler: authenticationHandler, api: api}
}

func (r *Router) AddUserRoutes() {
	huma.Register(r.api, huma.Operation{
		OperationID:   "create-user",
		Method:        http.MethodPost,
		Path:          "/users",
		Summary:       "Create a new user",
		Tags:          []string{"Users"},
		DefaultStatus: http.StatusCreated,
	}, r.UserHandler.CreateUser)

	huma.Register(r.api, huma.Operation{
		OperationID: "get-user",
		Method:      http.MethodGet,
		Path:        "/users/{id}",
		Summary:     "Get a user by ID",
		Tags:        []string{"Users"},
	}, r.UserHandler.GetUser)
}

func (r *Router) AddAuthRoutes() {
	huma.Register(r.api, huma.Operation{
		OperationID:   "register",
		Method:        http.MethodPost,
		Path:          "/register",
		Summary:       "Register a new user",
		Tags:          []string{"Authentication"},
		DefaultStatus: http.StatusCreated,
	}, r.AuthenticationHandler.RegisterUser)

	huma.Register(r.api, huma.Operation{
		OperationID:   "login",
		Method:        http.MethodPost,
		Path:          "/login",
		Summary:       "Login with credentials",
		Tags:          []string{"Authentication"},
		DefaultStatus: http.StatusOK,
	}, r.AuthenticationHandler.LoginUser)
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
