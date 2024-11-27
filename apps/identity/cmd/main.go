package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/danielgtaylor/huma/v2/humacli"
	"github.com/go-chi/chi/v5"

	_ "github.com/danielgtaylor/huma/v2/formats/cbor"
)

type User struct {
	ID       string `json:"id" doc:"Unique identifier for the user"`
	Username string `json:"username" maxLength:"30" doc:"Username for the user"`
	Email    string `json:"email" format:"email" doc:"Email address of the user"`
}

// UsersDB simulates a simple in-memory data store.
var UsersDB = map[string]User{}

type Options struct {
	Port int `help:"Port to listen on" short:"p" default:"8888"`
}

type CreateUserRequest struct {
	Body struct {
		Username string `json:"username" required:"true" maxLength:"30" doc:"Username for the user"`
		Email    string `json:"email" required:"true" format:"email" doc:"Email address of the user"`
	}
}

type CreateUserResponse struct {
	Body User `json:"body" doc:"The created user"`
}

var startTime = time.Now()

type GetUserResponse struct {
	Body User `json:"body" doc:"The requested user"`
}

type HealthCheckResponse struct {
	Body struct {
		Status string `json:"status" doc:"Status of the service"`
		Uptime string `json:"uptime" doc:"Time the service has been running"`
	}
}

func addRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "health-check",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Health check",
		Tags:        []string{"Health"},
	}, func(ctx context.Context, request *struct{}) (*HealthCheckResponse, error) {
		uptime := time.Since(startTime).String()

		resp := &HealthCheckResponse{}
		resp.Body.Status = "OK"
		resp.Body.Uptime = uptime

		return resp, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-user",
		Method:        http.MethodPost,
		Path:          "/users",
		Summary:       "Create a new user",
		Tags:          []string{"Users"},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, input *CreateUserRequest) (*CreateUserResponse, error) {
		userID := fmt.Sprintf("user-%d", len(UsersDB)+1)

		user := User{
			ID:       userID,
			Username: input.Body.Username,
			Email:    input.Body.Email,
		}
		UsersDB[userID] = user

		resp := &CreateUserResponse{}
		resp.Body.ID = user.ID
		resp.Body.Email = user.Email
		resp.Body.Username = user.Username

		return resp, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-user",
		Method:      http.MethodGet,
		Path:        "/users/{id}",
		Summary:     "Get a user by ID",
		Tags:        []string{"Users"},
	}, func(ctx context.Context, input *struct {
		ID string `path:"id" doc:"ID of the user to retrieve"`
	}) (*GetUserResponse, error) {
		user, exists := UsersDB[input.ID]
		if !exists {
			return nil, huma.NewError(http.StatusNotFound, "User not found")
		}

		return &GetUserResponse{
			Body: user,
		}, nil
	})
}

func main() {
	// Create a CLI app which takes a port option.
	cli := humacli.New(func(hooks humacli.Hooks, options *Options) {
		// Create a new router & API
		router := chi.NewMux()
		api := humachi.New(router, huma.DefaultConfig("My API", "1.0.0"))

		addRoutes(api)

		// Tell the CLI how to start your server.
		hooks.OnStart(func() {
			fmt.Printf("Starting server on port %d...\n", options.Port)
			err := http.ListenAndServe(fmt.Sprintf(":%d", options.Port), router)
			if err != nil {
				log.Fatal("Router failed to start")
			}
		})
	})

	// Run the CLI. When passed no commands, it starts the server.
	cli.Run()
}
