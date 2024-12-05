package repo

import (
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/tanjd/core-repository/apps/identity/model"
)

type EmailVerificationData struct {
	UserID     uuid.UUID
	Expiration time.Time
}

type InMemoryRepo struct {
	users                   map[uuid.UUID]model.User
	emailVerificationTokens map[string]EmailVerificationData
}

func NewInMemoryRepo() *InMemoryRepo {
	return &InMemoryRepo{
		users:                   make(map[uuid.UUID]model.User),
		emailVerificationTokens: make(map[string]EmailVerificationData),
	}
}

func (r *InMemoryRepo) CreateUser(user *model.User) (*model.User, error) {
	r.users[user.ID] = *user

	log.Info().
		Interface("users", r.users).
		Msg("Create User")
	return user, nil
}

func (r *InMemoryRepo) GetUser(id uuid.UUID) (*model.User, error) {
	user, exists := r.users[id]
	if !exists {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

func (r *InMemoryRepo) GetUserByEmail(email string) (*model.User, error) {
	for _, user := range r.users {
		if user.Email == email {
			return &user, nil
		}
	}
	return nil, ErrUserNotFound
}

func (r *InMemoryRepo) GetUserByUsername(username string) (*model.User, error) {
	for _, user := range r.users {
		if user.Username == username {
			return &user, nil
		}
	}
	return nil, ErrUserNotFound
}
