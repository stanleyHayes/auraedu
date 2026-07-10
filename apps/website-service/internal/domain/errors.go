package domain

import "errors"

var (
	ErrNotFound      = errors.New("website: not found")
	ErrValidation    = errors.New("website: validation failed")
	ErrMissingTenant = errors.New("website: tenant context required")
	ErrForbidden     = errors.New("website: forbidden")
	ErrConflict      = errors.New("website: conflict")
)
