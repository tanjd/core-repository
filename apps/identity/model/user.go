package model

import "github.com/google/uuid"

type User struct {
	ID         uuid.UUID `json:"id" doc:"Unique identifier for the user"`
	Username   string    `json:"username" doc:"Username for the user"`
	Email      string    `json:"email" doc:"Email address of the user"`
	Password   string    `json:"-" doc:"Hashed password"`
	IsVerified bool      `json:"is_verified" doc:"Indicates if the user's email is verified"`
}

type CreateUserRequest struct {
	Body struct {
		Username string `json:"username" required:"true" maxLength:"30" doc:"Username for the user. Must be unique and up to 30 characters long."`
		Email    string `json:"email" required:"true" format:"email" doc:"Valid email address for the user. This will be used for login and communication."`
		Password string `json:"password" required:"true" minLength:"8" doc:"Password for the user. Must be at least 8 characters long and contain at least one uppercase letter, one lowercase letter, one number, and one special character."`
	}
}

type CreateUserResponse struct {
	Body User `json:"body" doc:"The created user"`
}

type GetUserRequest struct {
	ID string `path:"id" doc:"ID of the user to retrieve"`
}

type GetUserResponse struct {
	Body User `json:"body" doc:"The requested user"`
}
