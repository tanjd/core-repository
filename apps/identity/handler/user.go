package handler

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/tanjd/core-repository/apps/identity/model"
)

type UserRepo interface {
	CreateUser(username, email string) (*model.User, error)
	GetUser(id uuid.UUID) (*model.User, error)
}

type UserHandler struct {
	Repo UserRepo
}

func NewUserHandler(r UserRepo) *UserHandler {
	return &UserHandler{
		Repo: r,
	}
}

func (h UserHandler) CreateUserHandler(ctx context.Context, request *model.CreateUserRequest) (*model.CreateUserResponse, error) {
	user, err := h.Repo.CreateUser(request.Body.Username, request.Body.Email)
	if err != nil {
		return nil, err
	}
	resp := &model.CreateUserResponse{
		Body: *user,
	}
	return resp, nil
}

func (h UserHandler) GetUserHandler(ctx context.Context, request *struct {
	ID string `path:"id" doc:"ID of the user to retrieve"`
}) (*model.GetUserResponse, error) {
	userID, err := uuid.Parse(request.ID)
	if err != nil {
		return nil, huma.NewError(http.StatusBadRequest, "Invalid user ID format")
	}
	user, err := h.Repo.GetUser(userID)
	if err != nil {
		return nil, err
	}
	return &model.GetUserResponse{
		Body: *user,
	}, nil
}
