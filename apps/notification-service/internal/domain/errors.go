package domain

import "errors"

var (
	ErrNotFound      = errors.New("notifications: not found")
	ErrValidation    = errors.New("notifications: validation failed")
	ErrMissingTenant = errors.New("notifications: tenant context required")
	ErrForbidden     = errors.New("notifications: forbidden")
	ErrConflict      = errors.New("notifications: conflict")
	ErrUnavailable   = errors.New("notifications: dependency unavailable")
)
