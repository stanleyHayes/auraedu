package domain

import "errors"

var (
	ErrNotFound      = errors.New("file: not found")
	ErrValidation    = errors.New("file: validation failed")
	ErrMissingTenant = errors.New("file: tenant context required")
)
