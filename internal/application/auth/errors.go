package auth

import "errors"

var (
	ErrEmailPasswordRequired = errors.New("Email and password are required")
	ErrInvalidEmail          = errors.New("Invalid Email")
	ErrIncorrectPassword     = errors.New("Incorrect Password")
	ErrNotAuthenticated      = errors.New("Not authenticated")
)
