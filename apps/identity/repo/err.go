package repo

import "errors"

var ErrUserNotFound = errors.New("User not found")

var ErrVerificationTokenNotFound = errors.New("Verification token not found")
