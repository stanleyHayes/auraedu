// Package http adapts HTTP to the Identity use cases (no business logic here).
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/auraedu/identity-service/internal/application"
	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/tenancy"
	"github.com/auraedu/platform/auth"
)

type Handler struct {
	svc *application.Service
}

func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
	mux.HandleFunc("DELETE /api/v1/auth/sessions/{session_id}", h.revokeSession)
	mux.HandleFunc("GET /api/v1/auth/me", h.me)
	mux.HandleFunc("POST /api/v1/auth/forgot-password", h.forgotPassword)
	mux.HandleFunc("POST /api/v1/auth/reset-password", h.resetPassword)

	mux.HandleFunc("GET /api/v1/users", h.listUsers)
	mux.HandleFunc("POST /api/v1/users", h.createUser)
	mux.HandleFunc("GET /api/v1/users/{id}", h.getUser)
	mux.HandleFunc("PUT /api/v1/users/{id}", h.updateUser)
	mux.HandleFunc("DELETE /api/v1/users/{id}", h.deleteUser)
	mux.HandleFunc("POST /api/v1/users/{id}/roles", h.assignRole)

	mux.HandleFunc("POST /api/v1/users/invites", h.inviteUser)
	mux.HandleFunc("POST /api/v1/users/invites/{token}/accept", h.acceptInvite)

	mux.HandleFunc("GET /api/v1/permissions", h.listPermissions)
	mux.HandleFunc("GET /api/v1/roles", h.listRoles)
}

func (h *Handler) actor(r *http.Request) auth.Actor {
	return auth.FromHeaders(r.Header)
}

func (h *Handler) ctx(r *http.Request) context.Context {
	return tenancy.WithActor(r.Context(), h.actor(r))
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	access, refresh, user, expires, err := h.svc.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			writeErr(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", "could not issue tokens")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"refresh_token": refresh,
		"token_type":    "Bearer",
		"expires_at":    expires.UTC().Format(time.RFC3339),
		"user":          userDTO(user),
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var body refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	access, refreshToken, user, expires, err := h.svc.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "invalid or expired refresh token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_at":    expires.UTC().Format(time.RFC3339),
		"user":          userDTO(user),
	})
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var body logoutRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if err := h.svc.Logout(h.ctx(r), h.actor(r), body.RefreshToken); err != nil {
		writeErr(w, mapStatus(err), codeFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) revokeSession(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RevokeSession(h.ctx(r), h.actor(r), r.PathValue("session_id")); err != nil {
		writeErr(w, mapStatus(err), codeFor(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	claims, err := h.svc.Verify(token)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":        claims.Subject,
		"tenant_id":      claims.TenantID,
		"role":           claims.Role,
		"permissions":    claims.Permissions,
		"features_hash":  claims.FeaturesHash,
		"platform_admin": claims.Role == auth.RolePlatformSuperAdmin,
	})
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var body forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if err := h.svc.RequestPasswordReset(r.Context(), body.Email); err != nil {
		writeErr(w, mapStatus(err), codeFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "ok", "message": "if the email exists, a reset link was sent"})
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var body resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	if err := h.svc.ResetPassword(r.Context(), body.Token, body.NewPassword); err != nil {
		writeErr(w, mapStatus(err), codeFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "password reset successfully"})
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.ListUsers(h.ctx(r), h.actor(r))
	if err != nil {
		writeErr(w, mapStatus(err), codeFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": users, "next_cursor": nil})
}

type createUserRequest struct {
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	Password    string   `json:"password"`
	TenantID    string   `json:"tenant_id,omitempty"`
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var body createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	u, err := h.svc.CreateUser(h.ctx(r), h.actor(r), application.CreateUserInput{
		TenantID:    body.TenantID,
		Email:       body.Email,
		Name:        body.Name,
		Role:        body.Role,
		Permissions: body.Permissions,
		Password:    body.Password,
	})
	if err != nil {
		writeErr(w, mapStatus(err), codeFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, userDTO(u))
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	u, err := h.svc.GetUser(h.ctx(r), h.actor(r), r.PathValue("id"))
	if err != nil {
		writeErr(w, mapStatus(err), codeFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, userDTO(u))
}

type updateUserRequest struct {
	Name        *string            `json:"name,omitempty"`
	Role        *string            `json:"role,omitempty"`
	Permissions *[]string          `json:"permissions,omitempty"`
	Status      *domain.UserStatus `json:"status,omitempty"`
}

func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	var body updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	u, err := h.svc.UpdateUser(h.ctx(r), h.actor(r), r.PathValue("id"), application.UpdateUserInput{
		Name:        body.Name,
		Role:        body.Role,
		Permissions: body.Permissions,
		Status:      body.Status,
	})
	if err != nil {
		writeErr(w, mapStatus(err), codeFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, userDTO(u))
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteUser(h.ctx(r), h.actor(r), r.PathValue("id")); err != nil {
		writeErr(w, mapStatus(err), codeFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

type assignRoleRequest struct {
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

func (h *Handler) assignRole(w http.ResponseWriter, r *http.Request) {
	var body assignRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	u, err := h.svc.AssignRole(h.ctx(r), h.actor(r), r.PathValue("id"), body.Role, body.Permissions)
	if err != nil {
		writeErr(w, mapStatus(err), codeFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, userDTO(u))
}

type inviteRequest struct {
	TenantID    string   `json:"tenant_id,omitempty"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

func (h *Handler) inviteUser(w http.ResponseWriter, r *http.Request) {
	var body inviteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	token, err := h.svc.InviteUser(h.ctx(r), h.actor(r), application.InviteInput{
		TenantID:    body.TenantID,
		Email:       body.Email,
		Role:        body.Role,
		Permissions: body.Permissions,
	})
	if err != nil {
		writeErr(w, mapStatus(err), codeFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"invite_token": token})
}

type acceptInviteRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (h *Handler) acceptInvite(w http.ResponseWriter, r *http.Request) {
	var body acceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_error", "invalid request body")
		return
	}
	u, err := h.svc.AcceptInvite(r.Context(), r.PathValue("token"), body.Name, body.Password)
	if err != nil {
		writeErr(w, mapStatus(err), codeFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, userDTO(u))
}

func (h *Handler) listPermissions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"data": []string{
			"features.manage", "users.read", "users.create", "users.update", "roles.assign",
			"students.read", "students.create", "students.update", "students.delete",
			"staff.read", "staff.create", "staff.update",
			"academic.read", "academic.manage",
			"attendance.read", "attendance.mark",
			"assessments.read", "assessments.record_scores", "assessments.manage",
			"reports.read", "reports.publish",
			"fees.read", "fees.manage",
			"payments.read", "payments.initiate",
			"notifications.read", "notifications.send", "notifications.manage",
			"website.read", "website.manage",
			"files.read", "files.upload", "files.delete",
			"analytics.view",
			"billing.read", "billing.manage",
			"cbt.read", "cbt.author", "cbt.take", "cbt.grade",
			"audit.read",
		},
	})
}

func (h *Handler) listRoles(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"data": []map[string]string{
			{"role": "platform_super_admin", "scope": "all_tenants"},
			{"role": "school_admin", "scope": "single_tenant"},
			{"role": "principal", "scope": "single_tenant"},
			{"role": "academic_head", "scope": "single_tenant"},
			{"role": "accountant", "scope": "single_tenant"},
			{"role": "teacher", "scope": "assigned_classes_subjects"},
			{"role": "parent", "scope": "own_children"},
			{"role": "student", "scope": "own_records"},
		},
	})
}

func userDTO(u domain.User) map[string]any {
	return map[string]any{
		"id":          u.ID,
		"email":       u.Email,
		"name":        u.Name,
		"tenant_id":   u.TenantID,
		"role":        u.Role,
		"permissions": u.Permissions,
		"status":      u.Status,
	}
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
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, map[string]string{"code": errCode, "message": msg})
}

func codeFor(err error) string {
	switch {
	case errors.Is(err, domain.ErrForbidden):
		return "forbidden"
	case errors.Is(err, domain.ErrValidation):
		return "validation_error"
	case errors.Is(err, domain.ErrNotFound):
		return "not_found"
	case errors.Is(err, domain.ErrConflict):
		return "conflict"
	case errors.Is(err, domain.ErrExpiredToken):
		return "unauthorized"
	case errors.Is(err, domain.ErrInvalidCredentials):
		return "invalid_credentials"
	default:
		return "internal"
	}
}

func mapStatus(err error) int {
	switch {
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, domain.ErrValidation):
		return http.StatusUnprocessableEntity
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, domain.ErrExpiredToken), errors.Is(err, domain.ErrInvalidCredentials):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}
