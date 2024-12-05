package service

import (
	"errors"
	"fmt"

	"github.com/tanjd/core-repository/apps/identity/repo"
)

func checkUserExists(email, username string, r UserRepo) error {
	if _, err := r.GetUserByEmail(email); err == nil {
		return fmt.Errorf("user with email '%s' already exists", email)
	} else if !errors.Is(err, repo.ErrUserNotFound) {
		return fmt.Errorf("error checking email: %w", err)
	}

	if _, err := r.GetUserByUsername(username); err == nil {
		return fmt.Errorf("user with username '%s' already exists", username)
	} else if !errors.Is(err, repo.ErrUserNotFound) {
		return fmt.Errorf("error checking username: %w", err)
	}

	return nil
}
