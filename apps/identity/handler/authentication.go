package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/tanjd/core-repository/apps/identity/model"
	"github.com/tanjd/core-repository/apps/identity/service"
)

type AuthenticationService interface {
	RegisterUser(username, email, password string) (*model.User, error)
	VerifyEmail(token string) error
	// LoginUser(username, hashPassword string) (*model.User, error)

}

type AuthenticationHandler struct {
	AuthenticationService
}

func NewAuthenticationHandler(s AuthenticationService) *AuthenticationHandler {
	return &AuthenticationHandler{
		AuthenticationService: s,
	}
}

func (h *AuthenticationHandler) RegisterUser(ctx context.Context, request *model.RegisterUserRequest) (*model.RegisterUserResponse, error) {
	user, err := h.AuthenticationService.RegisterUser(request.Body.Username, request.Body.Email, request.Body.Password)
	if err != nil {
		return nil, err
	}
	resp := &model.RegisterUserResponse{
		Body: struct {
			ID       uuid.UUID "json:\"id\" doc:\"Unique identifier for the user\""
			Username string    "json:\"username\" doc:\"Username for the user\""
			Email    string    "json:\"email\" doc:\"Email address of the user\""
		}{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
		},
	}
	return resp, nil
}

func (h *AuthenticationHandler) VerifyEmail(ctx context.Context, request *model.VerifyEmailRequest) (*model.VerifyEmailResponse, error) {
	err := h.AuthenticationService.VerifyEmail(request.Token)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			return nil, huma.NewError(http.StatusBadRequest, "Invalid verification token.")
		}
		if errors.Is(err, service.ErrTokenExpired) {
			return nil, huma.NewError(http.StatusBadRequest, "The verification token has expired. Please request a new one.")
		}
		return nil, huma.NewError(http.StatusInternalServerError, "An unexpected error occurred.")
	}

	return &model.VerifyEmailResponse{
		Body: struct {
			Message string `json:"message" doc:"A message indicating the result of the email verification process."`
		}{
			Message: "Email successfully verified.",
		},
	}, nil
}

func (h *AuthenticationHandler) LoginUser(ctx context.Context, req *model.LoginUserRequest) (*model.LoginUserResponse, error) {

	// user, err := h.UserRepo.GetUserByEmail(req.Body.Email)
	// if err != nil {
	// 	if errors.Is(err, ErrUserNotFound) {
	// 		return nil, huma.NewError(http.StatusUnauthorized, "Invalid email or password")
	// 	}
	// 	return nil, err
	// }

	// // Verify password
	// if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Body.Password)); err != nil {
	// 	return nil, huma.NewError(http.StatusUnauthorized, "Invalid email or password")
	// }

	// // Generate JWT token
	// token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
	// 	"sub": user.ID.String(),
	// 	"exp": jwt.TimeFunc().Add(time.Hour * 24).Unix(), // 24 hours expiration
	// })
	// tokenString, err := token.SignedString([]byte("your-secret-key"))
	// if err != nil {
	// 	return nil, huma.NewError(http.StatusInternalServerError, "Failed to generate token")
	// }

	// Prepare response
	return &model.LoginUserResponse{
		// Body: struct {
		// 	AccessToken  string `json:"access_token"`
		// 	RefreshToken string `json:"refresh_token,omitempty"`
		// 	ExpiresIn    int    `json:"expires_in"`
		// }{
		// 	AccessToken: tokenString,
		// 	ExpiresIn:   86400, // 24 hours
		// },
	}, nil
}
