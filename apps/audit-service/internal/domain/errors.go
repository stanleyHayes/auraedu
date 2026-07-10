package domain

import "errors"

var (
	ErrValidation    = errors.New("audit: validation failed")
	ErrMissingTenant = errors.New("audit: tenant context required")
)
