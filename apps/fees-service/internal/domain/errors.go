package domain

import "errors"

var (
	ErrNotFound      = errors.New("fees: not found")
	ErrValidation    = errors.New("fees: validation failed")
	ErrMissingTenant = errors.New("fees: tenant context required")
	ErrForbidden     = errors.New("fees: forbidden")
	ErrConflict      = errors.New("fees: conflict")
)
