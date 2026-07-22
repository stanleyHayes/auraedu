package domain

import "errors"

var (
	ErrNotFound      = errors.New("crm: not found")
	ErrValidation    = errors.New("crm: validation failed")
	ErrMissingTenant = errors.New("crm: tenant context required")
	ErrConflict      = errors.New("crm: idempotency conflict")
	ErrForbidden     = errors.New("crm: forbidden")
	ErrUnauthorized  = errors.New("crm: authentication required")
)
