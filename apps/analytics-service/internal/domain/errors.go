package domain

import "errors"

var (
	ErrNotFound      = errors.New("analytics: not found")
	ErrValidation    = errors.New("analytics: validation failed")
	ErrMissingTenant = errors.New("analytics: tenant context required")
	ErrForbidden     = errors.New("analytics: forbidden")
	ErrConflict      = errors.New("analytics: conflict")
)
