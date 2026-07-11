// Package domain contains the staff service aggregates and domain errors.
package domain

import "errors"

var (
	ErrNotFound      = errors.New("staff: not found")
	ErrValidation    = errors.New("staff: validation failed")
	ErrMissingTenant = errors.New("staff: tenant context required")
	ErrForbidden     = errors.New("staff: forbidden")
)
