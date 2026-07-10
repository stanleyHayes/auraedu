package domain

import "errors"

// ErrInvalidCredentials is returned for BOTH an unknown email and a wrong password,
// so callers cannot enumerate accounts (spec §9.1 / security).
var (
	ErrInvalidCredentials = errors.New("identity: invalid email or password")
	ErrUnauthenticated    = errors.New("identity: not authenticated")
)
