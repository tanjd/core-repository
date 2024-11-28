package repo

import (
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/tanjd/core-repository/apps/identity/model"
)

type InMemoryRepo struct {
	users map[uuid.UUID]model.User
}

func NewInMemoryRepo() *InMemoryRepo {
	return &InMemoryRepo{users: make(map[uuid.UUID]model.User)}
}

func (r *InMemoryRepo) CreateUser(username, email string) (*model.User, error) {
	if username == "" || email == "" {
		return nil, fmt.Errorf("username and email are required")
	}

	userID := uuid.New()
	user := model.User{
		ID:       userID,
		Username: username,
		Email:    email,
	}
	r.users[userID] = user
	return &user, nil
}

func (r *InMemoryRepo) GetUser(id uuid.UUID) (*model.User, error) {
	user, exists := r.users[id]
	if !exists {
		return nil, huma.NewError(http.StatusNotFound, "User not found")
	}
	return &user, nil
}
