package model

import "github.com/google/uuid"

type RegisterUserRequest struct {
	Body struct {
		Username string `json:"username" required:"true" maxLength:"30" doc:"Username for the user. Must be unique and up to 30 characters long."`
		Email    string `json:"email" required:"true" format:"email" doc:"Valid email address for the user. This will be used for login and communication."`
		Password string `json:"password" required:"true" minLength:"8" doc:"Password for the user. Must be at least 8 characters long and contain at least one uppercase letter, one lowercase letter, one number, and one special character."`
	}
}

type RegisterUserResponse struct {
	Body struct {
		ID       uuid.UUID `json:"id" doc:"Unique identifier for the user"`
		Username string    `json:"username" doc:"Username for the user"`
		Email    string    `json:"email" doc:"Email address of the user"`
	}
}

type LoginUserRequest struct {
	Body struct {
		Email    string `json:"email" required:"true" format:"email" doc:"Email address of the user"`
		Password string `json:"password" required:"true" doc:"Password for the user"`
	}
}

type LoginUserResponse struct {
	Body struct {
		AccessToken  string `json:"access_token" doc:"JWT access token for authentication"`
		RefreshToken string `json:"refresh_token,omitempty" doc:"JWT refresh token for renewing the access token"`
		ExpiresIn    int    `json:"expires_in" doc:"Duration (in seconds) until the token expires"`
	}
}

type VerifyEmailRequest struct {
	Token string `path:"token" doc:"The unique email verification token provided to the user during registration."`
}

type VerifyEmailResponse struct {
	Body struct {
		Message string `json:"message" doc:"A message indicating the result of the email verification process."`
	}
}
