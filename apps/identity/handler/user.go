package handler

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/tanjd/core-repository/apps/identity/model"
)

type UserService interface {
	CreateUser(username, email, password string) (*model.User, error)
	GetUser(id uuid.UUID) (*model.User, error)
}

type UserHandler struct {
	UserService
}

func NewUserHandler(s UserService) *UserHandler {
	return &UserHandler{
		UserService: s,
	}
}

func (h UserHandler) CreateUser(ctx context.Context, request *model.CreateUserRequest) (*model.CreateUserResponse, error) {
	user, err := h.UserService.CreateUser(request.Body.Username, request.Body.Email, request.Body.Password)
	if err != nil {
		return nil, err
	}
	resp := &model.CreateUserResponse{
		Body: *user,
	}
	return resp, nil
}

func (h UserHandler) GetUser(ctx context.Context, request *model.GetUserRequest) (*model.GetUserResponse, error) {
	userID, err := uuid.Parse(request.ID)
	if err != nil {
		return nil, huma.NewError(http.StatusBadRequest, "Invalid user ID format")
	}
	user, err := h.UserService.GetUser(userID)
	if err != nil {
		return nil, err
	}
	return &model.GetUserResponse{
		Body: *user,
	}, nil
}
