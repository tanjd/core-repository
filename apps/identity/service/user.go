package service

import (
	"errors"

	"github.com/google/uuid"
	"github.com/tanjd/core-repository/apps/identity/model"
	"golang.org/x/crypto/bcrypt"
)

type UserRepo interface {
	CreateUser(user *model.User) (*model.User, error)
	GetUser(id uuid.UUID) (*model.User, error)
	GetUserByEmail(email string) (*model.User, error)
	GetUserByUsername(username string) (*model.User, error)
}

type UserService struct {
	UserRepo
}

func NewUserService(r UserRepo) *UserService {
	return &UserService{
		UserRepo: r,
	}
}

func (s *UserService) CreateUser(username, email, password string) (*model.User, error) {
	if err := checkUserExists(email, username, s.UserRepo); err != nil {
		return nil, err
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("Failed to hash password")
	}
	user := &model.User{
		ID:         uuid.New(),
		Username:   username,
		Email:      email,
		Password:   string(hashedPassword),
		IsVerified: true,
	}
	if _, err := s.UserRepo.CreateUser(user); err != nil {
		return nil, errors.New("Failed to create user")
	}
	return user, nil
}

func (s *UserService) GetUser(id uuid.UUID) (*model.User, error) {
	user, err := s.UserRepo.GetUser(id)
	if err != nil {
		return nil, err
	}
	return user, nil
}
