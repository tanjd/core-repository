package service

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/tanjd/core-repository/apps/identity/model"
	"github.com/tanjd/core-repository/apps/identity/repo"
	"golang.org/x/crypto/bcrypt"
)

type EmailSender interface {
	SendEmailVerification(email, token string) error
}

type AuthenticationService struct {
	UserRepo
	EmailSender
	AuthenticationRepo
}

type AuthenticationRepo interface {
	StoreVerificationToken(userId uuid.UUID, token string, expiration time.Time) error
	RetrieveVerificationToken(token string) (*repo.EmailVerificationData, error)
	DeleteVerificationToken(token string) error
}

func NewAuthenticationService(userRepo UserRepo, emailSender EmailSender, authenticationRepo AuthenticationRepo) *AuthenticationService {
	return &AuthenticationService{
		UserRepo:           userRepo,
		EmailSender:        emailSender,
		AuthenticationRepo: authenticationRepo,
	}
}

func (s *AuthenticationService) RegisterUser(username, email, password string) (*model.User, error) {
	// @TODO: validate if email, username and password is valid

	if err := checkUserExists(email, username, s.UserRepo); err != nil {
		return nil, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}

	userId := uuid.New()

	user := &model.User{
		ID:         userId,
		Username:   username,
		Email:      email,
		Password:   string(hashedPassword),
		IsVerified: false,
	}
	if _, err := s.CreateUser(user); err != nil {
		return nil, errors.New("failed to create user")
	}

	verificationToken := uuid.New().String()

	expiration := time.Now().Add(24 * time.Hour) // Token valid for 24 hours
	if err := s.StoreVerificationToken(userId, verificationToken, expiration); err != nil {
		return nil, errors.New("failed to save verification token")
	}

	if err := s.SendEmailVerification(email, verificationToken); err != nil {
		return nil, errors.New("failed to send verification email")
	}

	return user, nil
}

func (s *AuthenticationService) VerifyEmail(token string) error {
	emailVerificationData, err := s.RetrieveVerificationToken(token)
	if err != nil {
		if errors.Is(err, repo.ErrVerificationTokenNotFound) {
			return ErrInvalidToken
		}
		return err
	}

	if isExpired(emailVerificationData.Expiration) {
		return ErrTokenExpired
	}

	updatedUser, err := s.UpdateUser(&model.UserUpdate{
		ID:         &emailVerificationData.UserID,
		IsVerified: boolPtr(true),
	})
	if err != nil {
		return err
	}

	err = s.DeleteVerificationToken(token)
	if err != nil {
		return err
	}

	log.Info().
		Interface("updatedUser", updatedUser).
		Msg("VerifyEmail")
	return nil
}

func boolPtr(b bool) *bool { return &b }

func isExpired(expiryTime time.Time) bool {
	return time.Now().After(expiryTime)
}
