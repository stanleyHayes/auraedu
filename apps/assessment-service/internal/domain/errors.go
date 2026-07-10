package domain

import "errors"

var (
	ErrNotFound      = errors.New("assessment: not found")
	ErrValidation    = errors.New("assessment: validation failed")
	ErrMissingTenant = errors.New("assessment: tenant context required")
	ErrForbidden     = errors.New("assessment: forbidden")
	ErrConflict      = errors.New("assessment: conflict")
)
