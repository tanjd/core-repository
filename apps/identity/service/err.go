package service

import "errors"

var ErrTokenExpired = errors.New("Token has expired")

var ErrInvalidToken = errors.New("Invalid Token")
