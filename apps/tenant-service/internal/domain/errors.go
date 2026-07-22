// Package domain contains the tenant service business rules and domain errors.
package domain

import "errors"

var (
	ErrNotFound    = errors.New("tenant: not found")
	ErrValidation  = errors.New("tenant: validation failed")
	ErrNoTenant    = errors.New("tenant: tenant context required")
	ErrForbidden   = errors.New("tenant: forbidden")
	ErrEntitlement = errors.New("tenant: plan does not include this feature")
	ErrConflict    = errors.New("tenant: conflicting state")
	ErrUnavailable = errors.New("tenant: dependency unavailable")
)
