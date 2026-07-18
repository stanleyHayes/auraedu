package domain

import "errors"

var (
	ErrValidation    = errors.New("audit: validation failed")
	ErrMissingTenant = errors.New("audit: tenant context required")
	ErrForbidden     = errors.New("audit: not permitted for this actor or tenant")
)
