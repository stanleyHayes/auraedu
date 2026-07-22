package domain

import "errors"

var (
	ErrNotFound      = errors.New("attendance: not found")
	ErrValidation    = errors.New("attendance: validation failed")
	ErrMissingTenant = errors.New("attendance: tenant context required")
	ErrForbidden     = errors.New("attendance: forbidden")
	ErrConflict      = errors.New("attendance: conflict")
	ErrUnavailable   = errors.New("attendance: dependency unavailable")
)
