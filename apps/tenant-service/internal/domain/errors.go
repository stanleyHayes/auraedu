package domain

import "errors"

var (
	ErrNotFound    = errors.New("tenant: not found")
	ErrValidation  = errors.New("tenant: validation failed")
	ErrNoTenant    = errors.New("tenant: tenant context required")
	ErrForbidden   = errors.New("tenant: forbidden")
	ErrEntitlement = errors.New("tenant: plan does not include this feature")
)
