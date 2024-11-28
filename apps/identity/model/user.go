package model

import "github.com/google/uuid"

type User struct {
	ID       uuid.UUID `json:"id" doc:"Unique identifier for the user"`
	Username string    `json:"username" maxLength:"30" doc:"Username for the user"`
	Email    string    `json:"email" format:"email" doc:"Email address of the user"`
}

type CreateUserRequest struct {
	Body struct {
		Username string `json:"username" required:"true" maxLength:"30" doc:"Username for the user"`
		Email    string `json:"email" required:"true" format:"email" doc:"Email address of the user"`
	}
}

type CreateUserResponse struct {
	Body User `json:"body" doc:"The created user"`
}

type GetUserResponse struct {
	Body User `json:"body" doc:"The requested user"`
}
