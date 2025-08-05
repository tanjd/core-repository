package repo

import "errors"

var ErrUserNotFound = errors.New("user not found")

var ErrVerificationTokenNotFound = errors.New("verification token not found")
