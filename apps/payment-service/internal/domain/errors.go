package domain

import "errors"

var (
	ErrNotFound      = errors.New("payments: not found")
	ErrValidation    = errors.New("payments: validation failed")
	ErrMissingTenant = errors.New("payments: tenant context required")
	ErrForbidden     = errors.New("payments: forbidden")
	ErrConflict      = errors.New("payments: conflict")
)
