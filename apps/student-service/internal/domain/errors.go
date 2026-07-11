// Package domain contains the student service aggregates and domain errors.
package domain

import "errors"

var (
	ErrNotFound      = errors.New("student: not found")
	ErrValidation    = errors.New("student: validation failed")
	ErrMissingTenant = errors.New("student: tenant context required")
	ErrForbidden     = errors.New("student: forbidden")
)
