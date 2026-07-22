// Package domain contains the report service aggregates and domain errors.
package domain

import "errors"

var (
	ErrNotFound      = errors.New("report: not found")
	ErrValidation    = errors.New("report: validation failed")
	ErrMissingTenant = errors.New("report: tenant context required")
	ErrForbidden     = errors.New("report: forbidden")
	ErrConflict      = errors.New("report: conflict")
	ErrUnavailable   = errors.New("report: dependency unavailable")
)
