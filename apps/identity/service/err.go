package service

import "errors"

var ErrTokenExpired = errors.New("token has expired")

var ErrInvalidToken = errors.New("invalid token")
