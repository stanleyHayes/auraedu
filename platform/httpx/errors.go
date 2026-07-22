package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

type ErrorCode string

const (
	ErrForbidden       ErrorCode = "forbidden"
	ErrFeatureDisabled ErrorCode = "feature_disabled"
	ErrTenantMismatch  ErrorCode = "tenant_mismatch"
	ErrValidation      ErrorCode = "validation_error"
	ErrNotFound        ErrorCode = "not_found"
	ErrUnauthorized    ErrorCode = "unauthorized"
	ErrInternal        ErrorCode = "internal_error"
	ErrTooManyRequests ErrorCode = "too_many_requests"
	ErrPayloadTooLarge ErrorCode = "payload_too_large"
)

type Error struct {
	Code      ErrorCode      `json:"error"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

func (e Error) Error() string { return string(e.Code) + ": " + e.Message }

func RespondError(w http.ResponseWriter, r *http.Request, err Error) {
	status := StatusForError(err.Code)
	RespondJSON(w, r, status, err)
}

func RespondJSON(w http.ResponseWriter, _ *http.Request, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func StatusForError(code ErrorCode) int {
	switch code {
	case ErrUnauthorized:
		return http.StatusUnauthorized
	case ErrForbidden, ErrFeatureDisabled, ErrTenantMismatch:
		return http.StatusForbidden
	case ErrValidation:
		return http.StatusUnprocessableEntity
	case ErrNotFound:
		return http.StatusNotFound
	case ErrTooManyRequests:
		return http.StatusTooManyRequests
	case ErrPayloadTooLarge:
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}

func ErrorFrom(err error) Error {
	switch {
	case errors.Is(err, tenancy.ErrMissingTenant), errors.Is(err, tenancy.ErrTenantMismatch):
		return Error{Code: ErrTenantMismatch, Message: "tenant context required or mismatched"}
	case errors.Is(err, flags.ErrFeatureDisabled):
		return Error{Code: ErrFeatureDisabled, Message: "feature is disabled for this tenant"}
	default:
		return Error{Code: ErrInternal, Message: "internal server error"}
	}
}

func Forbidden(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		message = "permission denied"
	}
	RespondError(w, r, Error{Code: ErrForbidden, Message: message})
}

func FeatureDisabled(w http.ResponseWriter, r *http.Request, featureKey string) {
	RespondError(w, r, Error{
		Code:    ErrFeatureDisabled,
		Message: "feature is disabled for this tenant",
		Details: map[string]any{"feature": featureKey},
	})
}

func TenantMismatch(w http.ResponseWriter, r *http.Request) {
	RespondError(w, r, Error{Code: ErrTenantMismatch, Message: "tenant context required or mismatched"})
}

func ValidationError(w http.ResponseWriter, r *http.Request, details map[string]any) {
	RespondError(w, r, Error{Code: ErrValidation, Message: "validation failed", Details: details})
}

func NotFound(w http.ResponseWriter, r *http.Request, resource string) {
	RespondError(w, r, Error{Code: ErrNotFound, Message: resource + " not found"})
}

func Unauthorized(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		message = "authentication required"
	}
	RespondError(w, r, Error{Code: ErrUnauthorized, Message: message})
}

func InternalError(w http.ResponseWriter, r *http.Request, message string) {
	RespondError(w, r, Error{Code: ErrInternal, Message: message})
}

func PayloadTooLarge(w http.ResponseWriter, r *http.Request) {
	RespondError(w, r, Error{Code: ErrPayloadTooLarge, Message: "request body too large"})
}
