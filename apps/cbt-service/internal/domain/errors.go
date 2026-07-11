// Package domain contains the CBT aggregates and value objects.
package domain

import "errors"

var (
	ErrNotFound      = errors.New("cbt: not found")
	ErrValidation    = errors.New("cbt: validation failed")
	ErrMissingTenant = errors.New("cbt: tenant context required")
	ErrForbidden     = errors.New("cbt: forbidden")
	ErrConflict      = errors.New("cbt: conflict")
)
