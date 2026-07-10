// Package memory is an in-memory Repository implementation for fast unit tests.
package memory

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/ports"
	"github.com/auraedu/identity-service/internal/tenancy"
	"github.com/google/uuid"
)

type entry struct {
	user domain.User
	cred domain.Credential
}

type refreshToken struct {
	userID    string
	expiresAt time.Time
	revokedAt *time.Time
}

type resetToken struct {
	userID    string
	tenantID  string
	expiresAt time.Time
	usedAt    *time.Time
}

type invite struct {
	ports.InviteDetails
	expiresAt time.Time
	usedAt    *time.Time
}

type Repository struct {
	mu      sync.RWMutex
	byEmail map[string]entry
	byID    map[string]entry
	refresh map[string]refreshToken
	resets  map[string]resetToken
	invites map[string]invite
	now     func() time.Time
}

var _ ports.Repository = (*Repository)(nil)

const demoPassword = "password123"

func New() (*Repository, error) {
	r := &Repository{
		byEmail: make(map[string]entry),
		byID:    make(map[string]entry),
		refresh: make(map[string]refreshToken),
		resets:  make(map[string]resetToken),
		invites: make(map[string]invite),
		now:     time.Now,
	}
	seed := []domain.User{
		{ID: "u-teacher", Email: "e.mensah@upshs.edu.gh", Name: "Efua Mensah", TenantID: "upshs", Role: "teacher",
			Permissions: []string{"attendance.mark", "assessments.record_scores", "reports.read"}},
		{ID: "u-admin", Email: "admin@upshs.edu.gh", Name: "School Admin", TenantID: "upshs", Role: "school_admin",
			Permissions: []string{"features.manage", "students.create", "students.update", "staff.create", PermUsersCreate, PermUsersRead, PermUsersUpdate, PermRolesAssign}},
		{ID: "u-super", Email: "super@auraedu.dev", Name: "Platform Admin", TenantID: "", Role: "platform_super_admin"},
	}
	for _, u := range seed {
		cred, err := domain.NewCredential(demoPassword)
		if err != nil {
			return nil, err
		}
		r.byEmail[strings.ToLower(u.Email)] = entry{user: u, cred: cred}
		r.byID[u.ID] = entry{user: u, cred: cred}
	}
	return r, nil
}

func (r *Repository) WithClock(now func() time.Time) *Repository { r.now = now; return r }

func (r *Repository) actor(ctx context.Context) (string, bool) {
	a := tenancy.ActorFromContext(ctx)
	if a.PlatformAdmin {
		return "", true
	}
	return a.TenantID, false
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (domain.User, domain.Credential, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return domain.User{}, domain.Credential{}, false, nil
	}
	return e.user, e.cred, true, nil
}

func (r *Repository) CreateUser(ctx context.Context, u domain.User, cred domain.Credential) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	email := strings.ToLower(strings.TrimSpace(u.Email))
	if _, exists := r.byEmail[email]; exists {
		return "", domain.ErrConflict
	}
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	u.Email = email
	r.byEmail[email] = entry{user: u, cred: cred}
	r.byID[u.ID] = entry{user: u, cred: cred}
	return u.ID, nil
}

func (r *Repository) ListUsers(ctx context.Context) ([]domain.User, error) {
	tenantID, admin := r.actor(ctx)
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []domain.User
	for _, e := range r.byID {
		if admin || e.user.TenantID == tenantID {
			out = append(out, e.user)
		}
	}
	return out, nil
}

func (r *Repository) GetUser(ctx context.Context, id string) (domain.User, error) {
	tenantID, admin := r.actor(ctx)
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	if !admin && e.user.TenantID != tenantID {
		return domain.User{}, domain.ErrNotFound
	}
	return e.user, nil
}

func (r *Repository) UpdateUser(ctx context.Context, id string, u domain.User) error {
	tenantID, admin := r.actor(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	if !admin && e.user.TenantID != tenantID {
		return domain.ErrNotFound
	}
	if u.Name != "" {
		e.user.Name = u.Name
	}
	if u.Role != "" {
		e.user.Role = u.Role
	}
	if u.Permissions != nil {
		e.user.Permissions = u.Permissions
	}
	if u.Status != "" {
		e.user.Status = u.Status
	}
	r.byID[id] = e
	r.byEmail[strings.ToLower(e.user.Email)] = e
	return nil
}

func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	tenantID, admin := r.actor(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	if !admin && e.user.TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(r.byID, id)
	delete(r.byEmail, strings.ToLower(e.user.Email))
	return nil
}

func (r *Repository) UpdateCredential(ctx context.Context, userID string, cred domain.Credential) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[userID]
	if !ok {
		return domain.ErrNotFound
	}
	e.cred = cred
	r.byID[userID] = e
	r.byEmail[strings.ToLower(e.user.Email)] = e
	return nil
}

func (r *Repository) SaveRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refresh[tokenHash] = refreshToken{userID: userID, expiresAt: expiresAt}
	return nil
}

func (r *Repository) FindRefreshToken(ctx context.Context, tokenHash string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.refresh[tokenHash]
	if !ok || t.revokedAt != nil || r.now().After(t.expiresAt) {
		return "", domain.ErrExpiredToken
	}
	return t.userID, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.refresh[tokenHash]
	if !ok {
		return domain.ErrExpiredToken
	}
	n := r.now()
	t.revokedAt = &n
	r.refresh[tokenHash] = t
	return nil
}

func (r *Repository) SavePasswordResetToken(ctx context.Context, tenantID, userID, tokenHash string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resets[tokenHash] = resetToken{userID: userID, tenantID: tenantID, expiresAt: expiresAt}
	return nil
}

func (r *Repository) UsePasswordResetToken(ctx context.Context, tokenHash string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.resets[tokenHash]
	if !ok || t.usedAt != nil || r.now().After(t.expiresAt) {
		return "", domain.ErrExpiredToken
	}
	n := r.now()
	t.usedAt = &n
	r.resets[tokenHash] = t
	return t.userID, nil
}

func (r *Repository) SaveInvite(ctx context.Context, tenantID, email, role string, permissions []string, tokenHash string, invitedBy *string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invites[tokenHash] = invite{
		InviteDetails: ports.InviteDetails{TenantID: tenantID, Email: strings.ToLower(strings.TrimSpace(email)), Role: role, Permissions: permissions},
		expiresAt:     expiresAt,
	}
	return nil
}

func (r *Repository) UseInvite(ctx context.Context, tokenHash string) (ports.InviteDetails, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.invites[tokenHash]
	if !ok || t.usedAt != nil || r.now().After(t.expiresAt) {
		return ports.InviteDetails{}, domain.ErrExpiredToken
	}
	n := r.now()
	t.usedAt = &n
	r.invites[tokenHash] = t
	return t.InviteDetails, nil
}

const (
	PermUsersCreate = "users.create"
	PermUsersRead   = "users.read"
	PermUsersUpdate = "users.update"
	PermRolesAssign = "roles.assign"
)
