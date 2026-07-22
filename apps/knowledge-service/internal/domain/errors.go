package domain

import "errors"

var (
	ErrNotFound   = errors.New("knowledge source not found")
	ErrValidation = errors.New("knowledge source validation failed")
	ErrForbidden  = errors.New("knowledge access forbidden")
	ErrConflict   = errors.New("knowledge lifecycle conflict")
)
