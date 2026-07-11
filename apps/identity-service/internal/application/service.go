// Package application holds the Identity Service use cases (agent_plan §5).
package application

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/jwt"
	"github.com/auraedu/identity-service/internal/ports"
	"github.com/auraedu/identity-service/internal/tenancy"
	"github.com/auraedu/platform/auth"
	"github.com/google/uuid"
)

const (
	PermUsersCreate = "users.create"
	PermUsersRead   = "users.read"
	PermUsersUpdate = "users.update"
	PermRolesAssign = "roles.assign"
)

type Service struct {
	repo        ports.Repository
	sessions    ports.SessionStore
	publisher   ports.EventPublisher
	signingKey  []byte
	accessTTL   time.Duration
	refreshTTL  time.Duration
	now         func() time.Time
	serviceName string
}

func NewService(
	repo ports.Repository,
	sessions ports.SessionStore,
	publisher ports.EventPublisher,
	signingKey []byte,
	accessTTL, refreshTTL time.Duration,
) *Service {
	return &Service{
		repo:        repo,
		sessions:    sessions,
		publisher:   publisher,
		signingKey:  signingKey,
		accessTTL:   accessTTL,
		refreshTTL:  refreshTTL,
		now:         time.Now,
		serviceName: "identity-service",
	}
}

func (s *Service) WithClock(now func() time.Time) *Service { s.now = now; return s }

func (s *Service) Login(ctx context.Context, email, password string) (accessToken, refreshToken string, user domain.User, expires time.Time, err error) {
	found, cred, ok, err := s.repo.FindByEmail(ctx, email)
	if err != nil || !ok {
		dummy, credErr := domain.NewCredential("timing-equalizer")
		if credErr == nil {
			dummy.Verify(password)
		}
		return "", "", domain.User{}, time.Time{}, domain.ErrInvalidCredentials
	}
	if !cred.Verify(password) {
		return "", "", domain.User{}, time.Time{}, domain.ErrInvalidCredentials
	}
	return s.issueSession(ctx, found)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, user domain.User, expires time.Time, err error) {
	if refreshToken == "" {
		return "", "", domain.User{}, time.Time{}, domain.ErrExpiredToken
	}
	tokenHash := domain.HashToken(refreshToken)
	userID, err := s.repo.FindRefreshToken(ctx, tokenHash)
	if err != nil {
		return "", "", domain.User{}, time.Time{}, domain.ErrExpiredToken
	}
	if err := s.repo.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return "", "", domain.User{}, time.Time{}, err
	}
	// Refresh tokens authenticate on their own; load the user without tenant filtering.
	privileged := tenancy.WithActor(ctx, auth.Actor{PlatformAdmin: true})
	u, err := s.repo.GetUser(privileged, userID)
	if err != nil {
		return "", "", domain.User{}, time.Time{}, domain.ErrExpiredToken
	}
	if s.sessions != nil {
		if err := s.sessions.Revoke(ctx, u.TenantID, tokenHash); err != nil {
			slog.Default().ErrorContext(ctx, "failed to revoke old session", "err", err)
		}
	}
	return s.issueSession(ctx, u)
}

func (s *Service) Logout(ctx context.Context, actor auth.Actor, refreshToken string) error {
	if refreshToken == "" {
		return domain.ErrValidation
	}
	return s.revokeSessionByToken(ctx, actor, refreshToken)
}

func (s *Service) RevokeSession(ctx context.Context, actor auth.Actor, sessionID string) error {
	if sessionID == "" {
		return domain.ErrValidation
	}
	return s.revokeSessionByHash(ctx, actor, sessionID)
}

func (s *Service) revokeSessionByToken(ctx context.Context, actor auth.Actor, refreshToken string) error {
	return s.revokeSessionByHash(ctx, actor, domain.HashToken(refreshToken))
}

func (s *Service) revokeSessionByHash(ctx context.Context, actor auth.Actor, tokenHash string) error {
	userID, err := s.repo.FindRefreshToken(ctx, tokenHash)
	if err != nil {
		return domain.ErrExpiredToken
	}
	privileged := tenancy.WithActor(ctx, auth.Actor{PlatformAdmin: true})
	u, err := s.repo.GetUser(privileged, userID)
	if err != nil {
		return err
	}
	if !actor.CanAccessTenant(u.TenantID) {
		return domain.ErrForbidden
	}
	if err := s.repo.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return err
	}
	if s.sessions != nil {
		if err := s.sessions.Revoke(ctx, u.TenantID, tokenHash); err != nil {
			slog.Default().ErrorContext(ctx, "failed to revoke session", "err", err)
		}
	}
	return nil
}

func (s *Service) Verify(token string) (jwt.Claims, error) {
	return jwt.Verify(token, s.signingKey, s.now())
}

func (s *Service) CreateUser(ctx context.Context, actor auth.Actor, input CreateUserInput) (domain.User, error) {
	ctx = tenancy.WithActor(ctx, actor)
	if !actor.Has(PermUsersCreate) {
		return domain.User{}, domain.ErrForbidden
	}
	tenantID := actor.TenantID
	if input.TenantID != "" && actor.PlatformAdmin {
		tenantID = input.TenantID
	}
	if tenantID == "" && !actor.PlatformAdmin {
		return domain.User{}, domain.ErrForbidden
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" || input.Name == "" || input.Role == "" {
		return domain.User{}, domain.ErrValidation
	}
	u := domain.User{
		Email:       email,
		Name:        input.Name,
		TenantID:    tenantID,
		Role:        input.Role,
		Permissions: input.Permissions,
		Status:      domain.StatusActive,
	}
	var cred domain.Credential
	if input.Password != "" {
		var err error
		cred, err = domain.NewCredential(input.Password)
		if err != nil {
			return domain.User{}, err
		}
	}
	id, err := s.repo.CreateUser(ctx, u, cred)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return domain.User{}, domain.ErrConflict
		}
		return domain.User{}, err
	}
	u.ID = id
	return u, nil
}

func (s *Service) ListUsers(ctx context.Context, actor auth.Actor) ([]domain.User, error) {
	ctx = tenancy.WithActor(ctx, actor)
	if !actor.Has(PermUsersRead) {
		return nil, domain.ErrForbidden
	}
	if actor.TenantID == "" && !actor.PlatformAdmin {
		return nil, domain.ErrForbidden
	}
	return s.repo.ListUsers(ctx)
}

func (s *Service) GetUser(ctx context.Context, actor auth.Actor, id string) (domain.User, error) {
	ctx = tenancy.WithActor(ctx, actor)
	if !actor.Has(PermUsersRead) {
		return domain.User{}, domain.ErrForbidden
	}
	u, err := s.repo.GetUser(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	if !actor.CanAccessTenant(u.TenantID) {
		return domain.User{}, domain.ErrForbidden
	}
	return u, nil
}

func (s *Service) UpdateUser(ctx context.Context, actor auth.Actor, id string, input UpdateUserInput) (domain.User, error) {
	ctx = tenancy.WithActor(ctx, actor)
	if !actor.Has(PermUsersUpdate) && !actor.Has(PermRolesAssign) {
		return domain.User{}, domain.ErrForbidden
	}
	existing, err := s.repo.GetUser(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	if !actor.CanAccessTenant(existing.TenantID) {
		return domain.User{}, domain.ErrForbidden
	}
	u := domain.User{}
	if input.Name != nil {
		u.Name = *input.Name
	}
	if input.Role != nil {
		u.Role = *input.Role
	}
	if input.Permissions != nil {
		u.Permissions = *input.Permissions
	}
	if input.Status != nil {
		u.Status = *input.Status
	}
	if err := s.repo.UpdateUser(ctx, id, u); err != nil {
		return domain.User{}, err
	}
	if input.Role != nil && *input.Role != existing.Role {
		if err := s.publisher.Publish(ctx, s.newEvent(existing.TenantID, "user.role_changed.v1", map[string]any{
			"user_id":       id,
			"previous_role": existing.Role,
			"new_role":      *input.Role,
		})); err != nil {
			return domain.User{}, err
		}
	}
	return s.repo.GetUser(ctx, id)
}

func (s *Service) DeleteUser(ctx context.Context, actor auth.Actor, id string) error {
	ctx = tenancy.WithActor(ctx, actor)
	if !actor.Has(PermUsersUpdate) {
		return domain.ErrForbidden
	}
	existing, err := s.repo.GetUser(ctx, id)
	if err != nil {
		return err
	}
	if !actor.CanAccessTenant(existing.TenantID) {
		return domain.ErrForbidden
	}
	return s.repo.DeleteUser(ctx, id)
}

func (s *Service) AssignRole(ctx context.Context, actor auth.Actor, id, role string, permissions []string) (domain.User, error) {
	ctx = tenancy.WithActor(ctx, actor)
	if !actor.Has(PermRolesAssign) {
		return domain.User{}, domain.ErrForbidden
	}
	existing, err := s.repo.GetUser(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	if !actor.CanAccessTenant(existing.TenantID) {
		return domain.User{}, domain.ErrForbidden
	}
	u := domain.User{Role: role, Permissions: permissions}
	if err := s.repo.UpdateUser(ctx, id, u); err != nil {
		return domain.User{}, err
	}
	if err := s.publisher.Publish(ctx, s.newEvent(existing.TenantID, "user.role_changed.v1", map[string]any{
		"user_id":       id,
		"previous_role": existing.Role,
		"new_role":      role,
		"permissions":   permissions,
	})); err != nil {
		return domain.User{}, err
	}
	return s.repo.GetUser(ctx, id)
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	u, _, ok, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	token, err := domain.RandomToken()
	if err != nil {
		return err
	}
	tokenHash := domain.HashToken(token)
	expires := s.now().Add(15 * time.Minute)
	if err := s.repo.SavePasswordResetToken(ctx, u.TenantID, u.ID, tokenHash, expires); err != nil {
		return err
	}
	return s.publisher.Publish(ctx, s.newEvent(u.TenantID, "notification.requested.v1", map[string]any{
		"channel":   "email",
		"recipient": u.Email,
		"template":  "password_reset",
		"payload": map[string]any{
			"user_id":         u.ID,
			"name":            u.Name,
			"reset_token":     token,
			"expires_minutes": 15,
		},
	}))
}

func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if token == "" || newPassword == "" {
		return domain.ErrValidation
	}
	tokenHash := domain.HashToken(token)
	userID, err := s.repo.UsePasswordResetToken(ctx, tokenHash)
	if err != nil {
		return domain.ErrExpiredToken
	}
	cred, err := domain.NewCredential(newPassword)
	if err != nil {
		return err
	}
	return s.repo.UpdateCredential(ctx, userID, cred)
}

func (s *Service) InviteUser(ctx context.Context, actor auth.Actor, input InviteInput) (string, error) {
	ctx = tenancy.WithActor(ctx, actor)
	if !actor.Has(PermUsersCreate) {
		return "", domain.ErrForbidden
	}
	tenantID := actor.TenantID
	if input.TenantID != "" && actor.PlatformAdmin {
		tenantID = input.TenantID
	}
	if tenantID == "" {
		return "", domain.ErrForbidden
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" || input.Role == "" {
		return "", domain.ErrValidation
	}
	token, err := domain.RandomToken()
	if err != nil {
		return "", err
	}
	tokenHash := domain.HashToken(token)
	expires := s.now().Add(7 * 24 * time.Hour)
	invitedBy := ""
	if actor.UserID != "" {
		invitedBy = actor.UserID
	}
	if err := s.repo.SaveInvite(ctx, tenantID, email, input.Role, input.Permissions, tokenHash, &invitedBy, expires); err != nil {
		return "", err
	}
	if err := s.publisher.Publish(ctx, s.newEvent(tenantID, "notification.requested.v1", map[string]any{
		"channel":   "email",
		"recipient": email,
		"template":  "user_invite",
		"payload": map[string]any{
			"tenant_id":    tenantID,
			"role":         input.Role,
			"invite_token": token,
			"expires_days": 7,
		},
	})); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) AcceptInvite(ctx context.Context, token, name, password string) (domain.User, error) {
	if token == "" || name == "" || password == "" {
		return domain.User{}, domain.ErrValidation
	}
	details, err := s.repo.UseInvite(ctx, domain.HashToken(token))
	if err != nil {
		return domain.User{}, domain.ErrExpiredToken
	}
	cred, err := domain.NewCredential(password)
	if err != nil {
		return domain.User{}, err
	}
	u := domain.User{
		Email:       details.Email,
		Name:        name,
		TenantID:    details.TenantID,
		Role:        details.Role,
		Permissions: details.Permissions,
		Status:      domain.StatusActive,
	}
	id, err := s.repo.CreateUser(ctx, u, cred)
	if err != nil {
		return domain.User{}, err
	}
	u.ID = id
	return u, nil
}

func (s *Service) issueSession(ctx context.Context, u domain.User) (string, string, domain.User, time.Time, error) {
	now := s.now()
	expires := now.Add(s.accessTTL)
	claims := jwt.Claims{
		Subject:      u.ID,
		TenantID:     u.TenantID,
		UserID:       u.ID,
		Role:         u.Role,
		Permissions:  u.Permissions,
		FeaturesHash: "",
		IssuedAt:     now.Unix(),
		ExpiresAt:    expires.Unix(),
	}
	accessToken, err := jwt.Sign(claims, s.signingKey)
	if err != nil {
		return "", "", domain.User{}, time.Time{}, err
	}
	refreshToken, err := domain.RandomToken()
	if err != nil {
		return "", "", domain.User{}, time.Time{}, err
	}
	refreshHash := domain.HashToken(refreshToken)
	if err := s.repo.SaveRefreshToken(ctx, u.ID, refreshHash, expires.Add(s.refreshTTL)); err != nil {
		return "", "", domain.User{}, time.Time{}, err
	}
	if s.sessions != nil {
		if err := s.sessions.Save(ctx, u.TenantID, u.ID, refreshHash, s.refreshTTL); err != nil {
			return "", "", domain.User{}, time.Time{}, err
		}
	}
	return accessToken, refreshToken, u, expires, nil
}

func (s *Service) newEvent(tenantID, eventType string, data map[string]any) ports.Event {
	return ports.Event{
		SpecVersion:     "1.0",
		Type:            eventType,
		Source:          s.serviceName,
		ID:              uuid.NewString(),
		Time:            s.now().UTC(),
		TenantID:        tenantID,
		DataContentType: "application/json",
		Data:            data,
	}
}

type CreateUserInput struct {
	TenantID    string
	Email       string
	Name        string
	Role        string
	Permissions []string
	Password    string
}

type UpdateUserInput struct {
	Name        *string
	Role        *string
	Permissions *[]string
	Status      *domain.UserStatus
}

type InviteInput struct {
	TenantID    string
	Email       string
	Role        string
	Permissions []string
}
