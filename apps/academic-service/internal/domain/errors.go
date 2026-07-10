package domain

import "errors"

var (
	ErrNotFound      = errors.New("academic: not found")
	ErrValidation    = errors.New("academic: validation failed")
	ErrMissingTenant = errors.New("academic: tenant context required")
)
