package domain

import "errors"

var (
	ErrInvalidCredentials = errors.New("identity: invalid email or password")
	ErrMFARequired        = errors.New("identity: multi-factor authentication required")
	ErrUnauthenticated    = errors.New("identity: not authenticated")
	ErrNotFound           = errors.New("identity: not found")
	ErrValidation         = errors.New("identity: validation failed")
	ErrForbidden          = errors.New("identity: forbidden")
	ErrExpiredToken       = errors.New("identity: token expired or revoked")
	ErrConflict           = errors.New("identity: conflict")
	ErrUnavailable        = errors.New("identity: dependent service unavailable")
)
