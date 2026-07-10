package domain

import "errors"

var (
	ErrInvalidCredentials = errors.New("identity: invalid email or password")
	ErrUnauthenticated    = errors.New("identity: not authenticated")
	ErrNotFound           = errors.New("identity: not found")
	ErrValidation         = errors.New("identity: validation failed")
	ErrForbidden          = errors.New("identity: forbidden")
	ErrExpiredToken       = errors.New("identity: token expired or revoked")
	ErrConflict           = errors.New("identity: conflict")
)
