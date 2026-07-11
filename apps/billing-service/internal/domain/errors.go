package domain

import "errors"

var (
	ErrNotFound      = errors.New("billing: not found")
	ErrValidation    = errors.New("billing: validation failed")
	ErrMissingTenant = errors.New("billing: tenant context required")
	ErrForbidden     = errors.New("billing: forbidden")
	ErrConflict      = errors.New("billing: conflict")
	ErrInvalidStatus = errors.New("billing: invalid status transition")
	ErrInvalidPlan   = errors.New("billing: invalid plan")
	ErrNoDefaultPlan = errors.New("billing: no default active plan")
)
