// Package memory is an in-memory Repository implementation for fast unit tests.
package memory

import (
	"context"
	"slices"
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
	familyID  string
	expiresAt time.Time
	revokedAt *time.Time
}

type resetToken struct {
	userID    string
	tenantID  string
	expiresAt time.Time
	usedAt    *time.Time
	revokedAt *time.Time
}

type invite struct {
	ports.InviteDetails
	expiresAt time.Time
	usedAt    *time.Time
	revokedAt *time.Time
}

type Repository struct {
	mu      sync.RWMutex
	byEmail map[string]entry
	byID    map[string]entry
	refresh map[string]refreshToken
	resets  map[string]resetToken
	invites map[string]invite
	mfa     map[string]ports.MFARecord
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
		mfa:     make(map[string]ports.MFARecord),
		now:     time.Now,
	}
	seed := []domain.User{
		{ID: "u-teacher", Email: "e.mensah@upshs.edu.gh", Name: "Efua Mensah", TenantID: "upshs", Role: "teacher",
			Permissions: []string{
				"students.read", "academic.read", "attendance.read", "attendance.mark",
				"assessments.read", "assessments.record_scores", "reports.read", "notifications.read",
				"ai.view_recommendations", "ai.approve_recommendations", "ai.view_predictions",
				"ai.approve_predictions", "ai.view_guidance", "ai.approve_guidance",
			}, Status: domain.StatusActive},
		{ID: "u-admin", Email: "admin@upshs.edu.gh", Name: "School Admin", TenantID: "upshs", Role: "school_admin",
			Permissions: []string{
				"features.manage", "students.create", "students.update", "staff.create",
				PermUsersCreate, PermUsersRead, PermUsersUpdate, PermRolesAssign,
			}, Status: domain.StatusActive},
		{ID: "u-super", Email: "super@auraedu.dev", Name: "Platform Admin", TenantID: "", Role: "platform_super_admin", Status: domain.StatusActive},
	}
	for _, u := range seed {
		cred, err := domain.NewCredential(demoPassword)
		if err != nil {
			return nil, err
		}
		r.byEmail[emailKey(u.TenantID, u.Email)] = entry{user: u, cred: cred}
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
	tenantID, _ := r.actor(ctx)
	e, ok := r.byEmail[emailKey(tenantID, email)]
	if !ok && tenantID != "" {
		e, ok = r.byEmail[emailKey("", email)]
	}
	if !ok {
		return domain.User{}, domain.Credential{}, false, nil
	}
	return e.user, e.cred, true, nil
}

func (r *Repository) CreateUser(_ context.Context, u domain.User, cred domain.Credential) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	email := strings.ToLower(strings.TrimSpace(u.Email))
	key := emailKey(u.TenantID, email)
	if _, exists := r.byEmail[key]; exists {
		return "", domain.ErrConflict
	}
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	u.Email = email
	r.byEmail[key] = entry{user: u, cred: cred}
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
	return r.updateUserLocked(id, u, tenantID, admin)
}

func (r *Repository) updateUserLocked(id string, u domain.User, tenantID string, admin bool) error {
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
	r.byEmail[emailKey(e.user.TenantID, e.user.Email)] = e
	return nil
}

func (r *Repository) UpdateUserAndRevokeSessions(ctx context.Context, id string, u domain.User) error {
	tenantID, admin := r.actor(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.updateUserLocked(id, u, tenantID, admin); err != nil {
		return err
	}
	now := r.now()
	for tokenHash, token := range r.refresh {
		if token.userID != id || token.revokedAt != nil {
			continue
		}
		token.revokedAt = &now
		r.refresh[tokenHash] = token
	}
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
	delete(r.byEmail, emailKey(e.user.TenantID, e.user.Email))
	return nil
}

func (r *Repository) ResetPasswordWithToken(_ context.Context, tokenHash, tenantID string, cred domain.Credential) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, ok := r.resets[tokenHash]
	if !ok || token.tenantID != tenantID || token.usedAt != nil || token.revokedAt != nil || !r.now().Before(token.expiresAt) {
		return domain.ErrExpiredToken
	}
	e, ok := r.byID[token.userID]
	if !ok {
		return domain.ErrNotFound
	}
	now := r.now()
	token.usedAt = &now
	r.resets[tokenHash] = token
	for hash, other := range r.resets {
		if hash == tokenHash || other.userID != token.userID || other.usedAt != nil || other.revokedAt != nil {
			continue
		}
		other.revokedAt = &now
		r.resets[hash] = other
	}
	e.cred = cred
	r.byID[token.userID] = e
	r.byEmail[emailKey(e.user.TenantID, e.user.Email)] = e
	for hash, refresh := range r.refresh {
		if refresh.userID != token.userID || refresh.revokedAt != nil {
			continue
		}
		refresh.revokedAt = &now
		r.refresh[hash] = refresh
	}
	return nil
}

func (r *Repository) GetMFA(_ context.Context, userID string) (ports.MFARecord, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.mfa[userID]
	if !ok {
		return ports.MFARecord{}, false, nil
	}
	record.EncryptedSecret = append([]byte(nil), record.EncryptedSecret...)
	return record, true, nil
}

func (r *Repository) SaveMFA(_ context.Context, userID string, encryptedSecret []byte, acceptedCounter int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[userID]; !ok {
		return domain.ErrNotFound
	}
	if _, enrolled := r.mfa[userID]; enrolled {
		return domain.ErrConflict
	}
	r.mfa[userID] = ports.MFARecord{EncryptedSecret: append([]byte(nil), encryptedSecret...), LastCounter: acceptedCounter}
	return nil
}

func (r *Repository) AdvanceMFACounter(_ context.Context, userID string, acceptedCounter int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	record, ok := r.mfa[userID]
	if !ok || acceptedCounter <= record.LastCounter {
		return domain.ErrInvalidCredentials
	}
	record.LastCounter = acceptedCounter
	r.mfa[userID] = record
	return nil
}

func (r *Repository) SaveRefreshToken(_ context.Context, userID, tokenHash, familyID string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[userID]; !ok {
		return domain.ErrNotFound
	}
	if familyID == "" {
		return domain.ErrValidation
	}
	if _, exists := r.refresh[tokenHash]; exists {
		return domain.ErrConflict
	}
	r.refresh[tokenHash] = refreshToken{userID: userID, familyID: familyID, expiresAt: expiresAt}
	return nil
}

func (r *Repository) FindRefreshToken(_ context.Context, tokenHash string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.refresh[tokenHash]
	if !ok {
		return "", domain.ErrExpiredToken
	}
	return t.userID, nil
}

func (r *Repository) RotateRefreshToken(_ context.Context, oldTokenHash, newTokenHash string, expiresAt time.Time) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, collision := r.refresh[newTokenHash]; collision {
		return "", domain.ErrConflict
	}
	token, ok := r.refresh[oldTokenHash]
	if !ok {
		return "", domain.ErrExpiredToken
	}
	if token.revokedAt != nil || !r.now().Before(token.expiresAt) {
		now := r.now()
		for hash, familyToken := range r.refresh {
			if familyToken.familyID == token.familyID && familyToken.revokedAt == nil {
				familyToken.revokedAt = &now
				r.refresh[hash] = familyToken
			}
		}
		return "", domain.ErrExpiredToken
	}
	now := r.now()
	token.revokedAt = &now
	r.refresh[oldTokenHash] = token
	r.refresh[newTokenHash] = refreshToken{
		userID: token.userID, familyID: token.familyID, expiresAt: expiresAt,
	}
	return token.userID, nil
}

func (r *Repository) RevokeRefreshFamily(_ context.Context, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.refresh[tokenHash]
	if !ok {
		return domain.ErrExpiredToken
	}
	n := r.now()
	for hash, token := range r.refresh {
		if token.familyID == t.familyID && token.revokedAt == nil {
			token.revokedAt = &n
			r.refresh[hash] = token
		}
	}
	return nil
}

func (r *Repository) RevokeUserSessions(_ context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[userID]; !ok {
		return domain.ErrNotFound
	}
	now := r.now()
	for tokenHash, token := range r.refresh {
		if token.userID != userID || token.revokedAt != nil {
			continue
		}
		token.revokedAt = &now
		r.refresh[tokenHash] = token
	}
	return nil
}

func (r *Repository) SavePasswordResetToken(_ context.Context, tenantID, userID, tokenHash string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	for hash, token := range r.resets {
		if token.tenantID != tenantID || token.userID != userID || token.usedAt != nil || token.revokedAt != nil {
			continue
		}
		token.revokedAt = &now
		r.resets[hash] = token
	}
	r.resets[tokenHash] = resetToken{userID: userID, tenantID: tenantID, expiresAt: expiresAt}
	return nil
}

func emailKey(tenantID, email string) string {
	return strings.ToLower(strings.TrimSpace(tenantID)) + "\x00" + strings.ToLower(strings.TrimSpace(email))
}

func (r *Repository) SaveInvite(_ context.Context, tenantID, email, role string, permissions []string, tokenHash string, _ *string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	now := r.now()
	for hash, existing := range r.invites {
		if existing.TenantID != tenantID || existing.Email != normalizedEmail || existing.usedAt != nil || existing.revokedAt != nil {
			continue
		}
		existing.revokedAt = &now
		r.invites[hash] = existing
	}
	r.invites[tokenHash] = invite{
		InviteDetails: ports.InviteDetails{TenantID: tenantID, Email: normalizedEmail, Role: role, Permissions: permissions},
		expiresAt:     expiresAt,
	}
	return nil
}

func (r *Repository) InspectInvite(_ context.Context, tokenHash string) (ports.InviteDetails, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.invites[tokenHash]
	if !ok || t.revokedAt != nil || !r.now().Before(t.expiresAt) {
		return ports.InviteDetails{}, domain.ErrExpiredToken
	}
	return t.InviteDetails, nil
}

func (r *Repository) AcceptInviteWithCredential(
	_ context.Context,
	tokenHash, name, password string,
	cred domain.Credential,
) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.invites[tokenHash]
	if !ok || t.revokedAt != nil || !r.now().Before(t.expiresAt) {
		return domain.User{}, domain.ErrExpiredToken
	}
	key := emailKey(t.TenantID, t.Email)
	e, found := r.byEmail[key]
	if !found {
		u := domain.User{
			ID:          uuid.NewString(),
			Email:       t.Email,
			Name:        strings.TrimSpace(name),
			TenantID:    t.TenantID,
			Role:        t.Role,
			Permissions: append([]string{}, t.Permissions...),
			Status:      domain.StatusActive,
		}
		e = entry{user: u, cred: cred}
	} else {
		if e.user.Role != t.Role || e.user.Status != domain.StatusActive ||
			!slices.Equal(e.user.Permissions, t.Permissions) {
			return domain.User{}, domain.ErrExpiredToken
		}
		if len(e.cred.Hash) == 0 {
			e.cred = cred
		} else if !e.cred.Verify(password) {
			return domain.User{}, domain.ErrExpiredToken
		}
	}
	r.byEmail[key] = e
	r.byID[e.user.ID] = e
	if t.usedAt == nil {
		now := r.now()
		t.usedAt = &now
		r.invites[tokenHash] = t
	}
	return e.user, nil
}

const (
	PermUsersCreate = "users.create"
	PermUsersRead   = "users.read"
	PermUsersUpdate = "users.update"
	PermRolesAssign = "roles.assign"
)
