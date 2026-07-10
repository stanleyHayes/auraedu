// Package http adapts HTTP to the Identity use cases (no business logic here).
// POST /api/v1/auth/login issues a JWT; GET /api/v1/auth/me verifies a bearer token.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/auraedu/identity-service/internal/application"
	"github.com/auraedu/identity-service/internal/domain"
)

type Handler struct {
	svc *application.Service
}

func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("GET /api/v1/auth/me", h.me)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errEnv("validation_error", "invalid request body"))
		return
	}
	token, user, expires, err := h.svc.Login(body.Email, body.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			writeJSON(w, http.StatusUnauthorized, errEnv("invalid_credentials", "invalid email or password"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errEnv("internal", "could not sign token"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_at":   expires.UTC().Format(time.RFC3339),
		"user": map[string]any{
			"id":        user.ID,
			"name":      user.Name,
			"email":     user.Email,
			"tenant_id": user.TenantID,
			"role":      user.Role,
		},
	})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, errEnv("unauthorized", "missing bearer token"))
		return
	}
	actor, err := h.svc.Verify(token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errEnv("unauthorized", "invalid or expired token"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":        actor.UserID,
		"tenant_id":      actor.TenantID,
		"role":           actor.Role,
		"permissions":    actor.Permissions,
		"platform_admin": actor.PlatformAdmin,
	})
}

func bearerToken(r *http.Request) string {
	if after, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func errEnv(code, msg string) map[string]string {
	return map[string]string{"code": code, "message": msg}
}
