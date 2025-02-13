package repo

import (
	"errors"
	"reflect"
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

	log.Debug().
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

func (r *InMemoryRepo) UpdateUser(updateData *model.UserUpdate) (*model.User, error) {
	if updateData == nil || updateData.ID == nil {
		return nil, errors.New("updateData or updateData.ID cannot be nil")
	}

	existingUser, err := r.GetUser(*updateData.ID)
	if err != nil {
		return nil, err
	}

	existingStruct := reflect.ValueOf(existingUser).Elem()
	updateStruct := reflect.ValueOf(updateData).Elem() // FIX: Dereference pointer

	for i := 0; i < updateStruct.NumField(); i++ {
		field := updateStruct.Type().Field(i)
		updateValue := updateStruct.Field(i)

		if updateValue.Kind() == reflect.Ptr && !updateValue.IsNil() {
			targetField := existingStruct.FieldByName(field.Name)
			if targetField.CanSet() { // Prevent panic on unexported fields
				targetField.Set(updateValue.Elem())
			}
		}
	}

	r.users[*updateData.ID] = *existingUser

	log.Debug().
		Interface("users", r.users).
		Msg("Update User")

	return existingUser, nil
}

func (r *InMemoryRepo) GetUserByUsername(username string) (*model.User, error) {
	for _, user := range r.users {
		if user.Username == username {
			return &user, nil
		}
	}
	return nil, ErrUserNotFound
}
