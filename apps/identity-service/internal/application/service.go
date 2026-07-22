// Package application holds the Identity Service use cases (agent_plan §5).
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

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
	notifier    ports.TransactionalNotifier
	activator   ports.TenantActivator
	mfaKey      []byte
	mfaRequired bool
}

type Option func(*Service)

func WithTransactionalNotifier(notifier ports.TransactionalNotifier) Option {
	return func(s *Service) { s.notifier = notifier }
}

func WithTenantActivator(activator ports.TenantActivator) Option {
	return func(s *Service) { s.activator = activator }
}

func NewService(
	repo ports.Repository,
	sessions ports.SessionStore,
	publisher ports.EventPublisher,
	signingKey []byte,
	accessTTL, refreshTTL time.Duration,
	opts ...Option,
) *Service {
	s := &Service{
		repo:        repo,
		sessions:    sessions,
		publisher:   publisher,
		signingKey:  signingKey,
		accessTTL:   accessTTL,
		refreshTTL:  refreshTTL,
		now:         time.Now,
		serviceName: "identity-service",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) WithClock(now func() time.Time) *Service { s.now = now; return s }

func (s *Service) Login(ctx context.Context, email, password string) (accessToken, refreshToken string, user domain.User, expires time.Time, err error) {
	result, err := s.LoginStart(ctx, email, password)
	if err != nil {
		return "", "", domain.User{}, time.Time{}, err
	}
	if result.Status != "authenticated" {
		return "", "", domain.User{}, time.Time{}, domain.ErrMFARequired
	}
	return result.AccessToken, result.RefreshToken, result.User, result.Expires, nil
}

func (s *Service) authenticate(ctx context.Context, email, password string) (domain.User, error) {
	if !passwordWithinMaximum(password) {
		dummy, err := domain.NewCredential("bounded-timing-equalizer")
		if err == nil {
			dummy.Verify("bounded-invalid-password")
		}
		return domain.User{}, domain.ErrInvalidCredentials
	}
	found, cred, ok, err := s.repo.FindByEmail(ctx, email)
	if err != nil || !ok {
		dummy, credErr := domain.NewCredential("timing-equalizer")
		if credErr == nil {
			dummy.Verify(password)
		}
		return domain.User{}, domain.ErrInvalidCredentials
	}
	if !cred.Verify(password) {
		return domain.User{}, domain.ErrInvalidCredentials
	}
	if found.Status != domain.StatusActive {
		return domain.User{}, domain.ErrInvalidCredentials
	}
	return found, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, user domain.User, expires time.Time, err error) {
	if refreshToken == "" {
		return "", "", domain.User{}, time.Time{}, domain.ErrExpiredToken
	}
	tokenHash := domain.HashToken(refreshToken)
	var expectedUserID string
	if s.sessions != nil {
		expectedUserID, err = s.repo.FindRefreshToken(ctx, tokenHash)
		if err != nil {
			return "", "", domain.User{}, time.Time{}, domain.ErrExpiredToken
		}
		privileged := tenancy.WithActor(ctx, auth.Actor{PlatformAdmin: true})
		expectedUser, getErr := s.repo.GetUser(privileged, expectedUserID)
		if getErr != nil {
			return "", "", domain.User{}, time.Time{}, s.revokeRefreshFamily(ctx, tokenHash, domain.ErrExpiredToken)
		}
		storedUserID, ok, findErr := s.sessions.Find(ctx, expectedUser.TenantID, tokenHash)
		if findErr != nil {
			return "", "", domain.User{}, time.Time{}, fmt.Errorf("%w: session store lookup: %w", domain.ErrUnavailable, findErr)
		}
		if !ok || storedUserID != expectedUserID {
			return "", "", domain.User{}, time.Time{}, s.revokeRefreshFamily(ctx, tokenHash, domain.ErrExpiredToken)
		}
	}
	newRefreshToken, err = domain.RandomToken()
	if err != nil {
		return "", "", domain.User{}, time.Time{}, err
	}
	newTokenHash := domain.HashToken(newRefreshToken)
	userID, err := s.repo.RotateRefreshToken(ctx, tokenHash, newTokenHash, s.now().Add(s.refreshTTL))
	if err != nil {
		return "", "", domain.User{}, time.Time{}, domain.ErrExpiredToken
	}
	if expectedUserID != "" && userID != expectedUserID {
		return "", "", domain.User{}, time.Time{}, s.revokeRefreshFamily(ctx, newTokenHash, domain.ErrExpiredToken)
	}
	// Refresh tokens authenticate on their own; load the user without tenant filtering.
	privileged := tenancy.WithActor(ctx, auth.Actor{PlatformAdmin: true})
	u, err := s.repo.GetUser(privileged, userID)
	if err != nil {
		return "", "", domain.User{}, time.Time{}, s.revokeRefreshFamily(ctx, newTokenHash, domain.ErrExpiredToken)
	}
	if u.Status != domain.StatusActive {
		return "", "", domain.User{}, time.Time{}, s.revokeRefreshFamily(ctx, newTokenHash, domain.ErrExpiredToken)
	}
	accessToken, expires, err = s.issueAccessToken(u)
	if err != nil {
		return "", "", domain.User{}, time.Time{}, s.revokeRefreshFamily(ctx, newTokenHash, err)
	}
	if s.sessions != nil {
		if err := s.sessions.Revoke(ctx, u.TenantID, tokenHash); err != nil {
			slog.Default().ErrorContext(ctx, "failed to revoke old session", "err", err)
		}
		if err := s.sessions.Save(ctx, u.TenantID, u.ID, newTokenHash, s.refreshTTL); err != nil {
			return "", "", domain.User{}, time.Time{}, s.revokeRefreshFamily(ctx, newTokenHash, err)
		}
	}
	return accessToken, newRefreshToken, u, expires, nil
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
	if err := s.repo.RevokeRefreshFamily(ctx, tokenHash); err != nil {
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
	if !actor.Has(PermUsersCreate) || !actor.Has(PermRolesAssign) {
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
	permissions := input.Permissions
	if permissions == nil {
		permissions = []string{}
	}
	if err := validateAuthorizationGrant(actor, tenantID, input.Role, permissions); err != nil {
		return domain.User{}, err
	}
	u := domain.User{
		Email:       email,
		Name:        input.Name,
		TenantID:    tenantID,
		Role:        input.Role,
		Permissions: permissions,
		Status:      domain.StatusActive,
	}
	var cred domain.Credential
	if input.Password != "" {
		if !validNewPassword(input.Password) {
			return domain.User{}, domain.ErrValidation
		}
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
	wantsAuthorizationChange, err := validateUserUpdateRequest(actor, input)
	if err != nil {
		return domain.User{}, err
	}
	existing, err := s.repo.GetUser(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	if !actor.CanAccessTenant(existing.TenantID) {
		return domain.User{}, domain.ErrForbidden
	}
	u, newRole, newPermissions, authorizationChanged, statusChanged := prepareUserUpdate(existing, input)
	if wantsAuthorizationChange {
		if err := validateAuthorizationGrant(actor, existing.TenantID, newRole, newPermissions); err != nil {
			return domain.User{}, err
		}
	}
	if authorizationChanged {
		event := ports.RoleChangeEvent{
			TenantID: existing.TenantID, UserID: id,
			PreviousRole: existing.Role, NewRole: newRole,
		}
		if input.Permissions != nil {
			event.Permissions = *input.Permissions
		}
		if durable, ok := s.repo.(ports.DurableRoleChangeRepository); ok {
			if err := durable.UpdateUserWithRoleChange(ctx, id, u, event); err != nil {
				return domain.User{}, err
			}
			return s.repo.GetUser(ctx, id)
		}
	}
	if statusChanged {
		if durable, ok := s.repo.(ports.DurableUserSessionRepository); ok {
			if err := durable.UpdateUserAndRevokeSessions(ctx, id, u); err != nil {
				return domain.User{}, err
			}
			return s.repo.GetUser(ctx, id)
		}
	}
	if err := s.repo.UpdateUser(ctx, id, u); err != nil {
		return domain.User{}, err
	}
	if authorizationChanged || statusChanged {
		if err := s.repo.RevokeUserSessions(ctx, id); err != nil {
			return domain.User{}, err
		}
	}
	if authorizationChanged {
		payload := ports.RoleChangeEventData(ports.RoleChangeEvent{
			TenantID: existing.TenantID, UserID: id, PreviousRole: existing.Role,
			NewRole: newRole, Permissions: permissionValues(input.Permissions),
		})
		if err := s.publisher.Publish(ctx, s.newEvent(existing.TenantID, "user.role_changed.v1", payload)); err != nil {
			return domain.User{}, err
		}
	}
	return s.repo.GetUser(ctx, id)
}

func validateUserUpdateRequest(actor auth.Actor, input UpdateUserInput) (bool, error) {
	wantsAuthorizationChange := input.Role != nil || input.Permissions != nil
	wantsProfileChange := input.Name != nil || input.Status != nil
	switch {
	case wantsAuthorizationChange && !actor.Has(PermRolesAssign):
		return false, domain.ErrForbidden
	case wantsProfileChange && !actor.Has(PermUsersUpdate):
		return false, domain.ErrForbidden
	case !wantsAuthorizationChange && !wantsProfileChange:
		return false, domain.ErrValidation
	case input.Status != nil && !validUserStatus(*input.Status):
		return false, domain.ErrValidation
	default:
		return wantsAuthorizationChange, nil
	}
}

func prepareUserUpdate(existing domain.User, input UpdateUserInput) (domain.User, string, []string, bool, bool) {
	update := domain.User{}
	newRole, newPermissions := existing.Role, existing.Permissions
	if input.Name != nil {
		update.Name = *input.Name
	}
	if input.Role != nil {
		update.Role, newRole = *input.Role, *input.Role
	}
	if input.Permissions != nil {
		update.Permissions, newPermissions = *input.Permissions, *input.Permissions
	}
	if input.Status != nil {
		update.Status = *input.Status
	}
	permissionsChanged := input.Permissions != nil && !slices.Equal(*input.Permissions, existing.Permissions)
	authorizationChanged := newRole != existing.Role || permissionsChanged
	statusChanged := input.Status != nil && *input.Status != existing.Status
	return update, newRole, newPermissions, authorizationChanged, statusChanged
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
	grantPermissions := permissions
	if grantPermissions == nil {
		grantPermissions = existing.Permissions
	}
	if err := validateAuthorizationGrant(actor, existing.TenantID, role, grantPermissions); err != nil {
		return domain.User{}, err
	}
	if role == existing.Role && slices.Equal(grantPermissions, existing.Permissions) {
		return existing, nil
	}
	u := domain.User{Role: role, Permissions: permissions}
	event := ports.RoleChangeEvent{
		TenantID: existing.TenantID, UserID: id, PreviousRole: existing.Role,
		NewRole: role, Permissions: permissions,
	}
	if durable, ok := s.repo.(ports.DurableRoleChangeRepository); ok {
		if err := durable.UpdateUserWithRoleChange(ctx, id, u, event); err != nil {
			return domain.User{}, err
		}
		return s.repo.GetUser(ctx, id)
	}
	if err := s.repo.UpdateUser(ctx, id, u); err != nil {
		return domain.User{}, err
	}
	if err := s.repo.RevokeUserSessions(ctx, id); err != nil {
		return domain.User{}, err
	}
	payload := ports.RoleChangeEventData(event)
	if err := s.publisher.Publish(ctx, s.newEvent(existing.TenantID, "user.role_changed.v1", payload)); err != nil {
		return domain.User{}, err
	}
	return s.repo.GetUser(ctx, id)
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	u, _, ok, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return err
	}
	requestTenant := tenancy.ActorFromContext(ctx).TenantID
	if !ok || requestTenant == "" || u.TenantID != requestTenant {
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
	if s.notifier == nil {
		return domain.ErrUnavailable
	}
	return s.notifier.Deliver(ctx, u.TenantID, u.Email, "password_reset", map[string]any{
		"name": u.Name, "reset_token": token, "expires_minutes": 15,
	})
}

func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if token == "" || !validNewPassword(newPassword) {
		return domain.ErrValidation
	}
	tokenHash := domain.HashToken(token)
	cred, err := domain.NewCredential(newPassword)
	if err != nil {
		return err
	}
	tenantID := tenancy.ActorFromContext(ctx).TenantID
	if tenantID == "" {
		return domain.ErrExpiredToken
	}
	if err := s.repo.ResetPasswordWithToken(ctx, tokenHash, tenantID, cred); err != nil {
		if errors.Is(err, domain.ErrExpiredToken) {
			return domain.ErrExpiredToken
		}
		return err
	}
	return nil
}

func (s *Service) InviteUser(ctx context.Context, actor auth.Actor, input InviteInput) (string, error) {
	ctx = tenancy.WithActor(ctx, actor)
	if !actor.Has(PermUsersCreate) || !actor.Has(PermRolesAssign) {
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
	permissions := input.Permissions
	if permissions == nil {
		permissions = []string{}
	}
	if err := validateAuthorizationGrant(actor, tenantID, input.Role, permissions); err != nil {
		return "", err
	}
	token, err := domain.RandomToken()
	if err != nil {
		return "", err
	}
	tokenHash := domain.HashToken(token)
	expires := s.now().Add(7 * 24 * time.Hour)
	var invitedBy *string
	if actor.UserID != "" {
		value := actor.UserID
		invitedBy = &value
	}
	if err := s.repo.SaveInvite(ctx, tenantID, email, input.Role, permissions, tokenHash, invitedBy, expires); err != nil {
		return "", err
	}
	if s.notifier == nil {
		return "", domain.ErrUnavailable
	}
	if err := s.notifier.Deliver(ctx, tenantID, email, "user_invite", map[string]any{
		"role": input.Role, "invite_token": token, "expires_days": 7,
	}); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) AcceptInvite(ctx context.Context, token, name, password string) (domain.User, error) {
	if token == "" || name == "" || !validNewPassword(password) {
		return domain.User{}, domain.ErrValidation
	}
	tokenHash := domain.HashToken(token)
	details, err := s.repo.InspectInvite(ctx, tokenHash)
	if err != nil {
		return domain.User{}, domain.ErrExpiredToken
	}
	if err := validateAuthorizationGrant(auth.Actor{PlatformAdmin: true}, details.TenantID, details.Role, details.Permissions); err != nil {
		return domain.User{}, domain.ErrForbidden
	}
	ctx = tenancy.WithActor(ctx, auth.Actor{TenantID: details.TenantID})
	cred, err := domain.NewCredential(password)
	if err != nil {
		return domain.User{}, err
	}
	u, err := s.repo.AcceptInviteWithCredential(ctx, tokenHash, name, password, cred)
	if err != nil {
		return domain.User{}, err
	}
	if u.Role == "school_admin" {
		if s.activator == nil {
			return domain.User{}, domain.ErrUnavailable
		}
		if err := s.activator.Activate(ctx, u.TenantID); err != nil {
			return domain.User{}, domain.ErrUnavailable
		}
	}
	return u, nil
}

func validNewPassword(password string) bool {
	length := utf8.RuneCountInString(password)
	return length >= 12 && passwordWithinMaximum(password)
}

func passwordWithinMaximum(password string) bool {
	return utf8.RuneCountInString(password) <= 256 && len(password) <= 1024
}

func (s *Service) issueSession(ctx context.Context, u domain.User) (string, string, domain.User, time.Time, error) {
	accessToken, expires, err := s.issueAccessToken(u)
	if err != nil {
		return "", "", domain.User{}, time.Time{}, err
	}
	refreshToken, err := domain.RandomToken()
	if err != nil {
		return "", "", domain.User{}, time.Time{}, err
	}
	refreshHash := domain.HashToken(refreshToken)
	if err := s.repo.SaveRefreshToken(ctx, u.ID, refreshHash, uuid.NewString(), s.now().Add(s.refreshTTL)); err != nil {
		return "", "", domain.User{}, time.Time{}, err
	}
	if s.sessions != nil {
		if err := s.sessions.Save(ctx, u.TenantID, u.ID, refreshHash, s.refreshTTL); err != nil {
			return "", "", domain.User{}, time.Time{}, s.revokeRefreshFamily(ctx, refreshHash, err)
		}
	}
	return accessToken, refreshToken, u, expires, nil
}

func (s *Service) revokeRefreshFamily(ctx context.Context, tokenHash string, cause error) error {
	if err := s.repo.RevokeRefreshFamily(ctx, tokenHash); err != nil {
		return errors.Join(cause, fmt.Errorf("identity: revoke refresh family: %w", err))
	}
	return cause
}

func (s *Service) issueAccessToken(u domain.User) (string, time.Time, error) {
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
		return "", time.Time{}, err
	}
	return accessToken, expires, nil
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

func permissionValues(values *[]string) []string {
	if values == nil {
		return nil
	}
	return *values
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
