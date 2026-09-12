// Package mongo persists each identity and all its security artifacts in one
// versioned aggregate. Compare-and-swap serializes security transitions across
// processes without relying on multi-document transactions or a process mutex.
package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/ports"
	identitytenancy "github.com/auraedu/identity-service/internal/tenancy"
	"github.com/auraedu/platform/auth"
	pm "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// This scope is reserved for platform identities, whose public TenantID is empty.
const platformIdentityScope = "__platform_identity__"
const collectionName = "identity_accounts"

type token struct {
	Hash    string
	Family  string
	Expires time.Time
	Used    *time.Time
	Revoked *time.Time
}
type invitation struct {
	Token   token
	Details ports.InviteDetails
}
type account struct {
	ID         string            `bson:"_id"`
	Tenant     string            `bson:"tenant_id"`
	Email      string            `bson:"email"`
	Version    int64             `bson:"version"`
	User       *domain.User      `bson:"user"`
	Credential domain.Credential `bson:"credential"`
	MFA        *ports.MFARecord  `bson:"mfa"`
	Refresh    []token           `bson:"refresh"`
	Resets     []token           `bson:"resets"`
	Invites    []invitation      `bson:"invites"`
	Deleted    bool              `bson:"deleted"`
}

type Repository struct {
	store  *pm.Store
	coll   *pm.Collection
	outbox *pm.ClaimableOutbox
	now    func() time.Time
}

var _ ports.Repository = (*Repository)(nil)
var _ ports.DurableRoleChangeRepository = (*Repository)(nil)
var _ ports.DurableUserSessionRepository = (*Repository)(nil)
var _ ports.AuthCleanupRepository = (*Repository)(nil)
var _ ports.OutboxRepository = (*Repository)(nil)

func NewRepository(store *pm.Store) *Repository {
	c := store.Collection(collectionName)
	return &Repository{store: store, coll: c, outbox: pm.NewClaimableOutbox(c, 5*time.Minute), now: time.Now}
}
func (r *Repository) EnsureIndexes(ctx context.Context) error {
	models := make([]driver.IndexModel, 0, 5)
	models = append(models, []driver.IndexModel{
		{Keys: bson.D{{Key: "tenant_id",
			Value: 1},
			{Key: "email",
				Value: 1}},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"deleted": false})},

		{Keys: bson.D{{Key: "user.id",
			Value: 1}},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"user.id": bson.M{"$type": "string"}})},
	}...)
	for _, key := range []string{"refresh.hash", "resets.hash", "invites.token.hash"} {
		models = append(models,
			driver.IndexModel{Keys: bson.D{{Key: key,
				Value: 1}},
				Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{key: bson.M{"$type": "string"}})})
	}
	_, err := r.store.Database().Collection(collectionName).Indexes().CreateMany(ctx, models)
	return err
}
func normalized(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
func tenantKey(tenant string) string {
	if tenant == "" {
		return platformIdentityScope
	}
	return tenant
}
func (r *Repository) scope(ctx context.Context) (*pm.Scope, error) {
	a := identitytenancy.ActorFromContext(ctx)
	if a.PlatformAdmin {
		return r.privileged(ctx)
	}
	if a.TenantID == "" || a.TenantID == platformIdentityScope {
		return nil, pm.ErrTenantRequired
	}
	return r.coll.ScopeTo(a.TenantID)
}

// Only credential/capability lookup and service-owned maintenance may use this
// internal scope, matching PostgreSQL's withPrivilegedTx. User CRUD uses scope.
func (r *Repository) privileged(ctx context.Context) (*pm.Scope, error) {
	return r.coll.Scope(auth.WithActor(tenancy.WithContext(ctx, tenancy.TenantContext{}), auth.Actor{PlatformAdmin: true}))
}
func (r *Repository) explicit(tenant string) (*pm.Scope, error) {
	return r.coll.ScopeTo(tenantKey(tenant))
}
func decode(result *driver.SingleResult) (account, error) {
	var a account
	err := result.Decode(&a)
	if errors.Is(err, driver.ErrNoDocuments) {
		err = domain.ErrNotFound
	}
	return a, err
}
func mapped(err error) error {
	if driver.IsDuplicateKeyError(err) {
		return domain.ErrConflict
	}
	return err
}
func (r *Repository) load(ctx context.Context, s *pm.Scope, q bson.M) (account, error) {
	q["deleted"] = bson.M{"$ne": true}
	return decode(s.FindOne(ctx, q))
}
func (r *Repository) mutate(ctx context.Context, s *pm.Scope, q bson.M, fn func(*account) error, events ...tenancy.CloudEvent) error {
	for attempt := 0; attempt < 100; attempt++ {
		a, err := r.load(ctx, s, q)
		if err != nil {
			return err
		}
		v := a.Version
		if err = fn(&a); err != nil {
			return err
		}
		update := bson.M{"$set": bson.M{"user": a.User,
			"credential": a.Credential,
			"mfa":        a.MFA,
			"refresh":    a.Refresh,
			"resets":     a.Resets,
			"invites":    a.Invites,
			"deleted":    a.Deleted},
			"$inc": bson.M{"version": 1}}
		res, err := s.UpdateWithEvents(ctx, bson.M{"_id": a.ID, "version": v}, update, events...)
		if err != nil {
			return mapped(err)
		}
		if res.MatchedCount == 1 {
			return nil
		}
		if err = ctx.Err(); err != nil {
			return err
		}
	}
	return fmt.Errorf("identity: concurrent modification retry limit exceeded")
}
func (r *Repository) FindByEmail(ctx context.Context, email string) (domain.User, domain.Credential, bool, error) {
	tenant := identitytenancy.ActorFromContext(ctx).TenantID
	for _, owner := range []string{tenant, ""} {
		s, scopeErr := r.explicit(owner)
		if scopeErr != nil {
			return domain.User{}, domain.Credential{}, false, scopeErr
		}
		a, err := r.load(ctx, s, bson.M{"email": normalized(email), "user": bson.M{"$ne": nil}})
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return domain.User{}, domain.Credential{}, false, err
		}
		return *a.User, a.Credential, true, nil
	}
	return domain.User{}, domain.Credential{}, false, nil
}
func (r *Repository) CreateUser(ctx context.Context, u domain.User, c domain.Credential) (string, error) {
	actor := identitytenancy.ActorFromContext(ctx)
	if !actor.PlatformAdmin && (u.TenantID == "" || actor.TenantID != u.TenantID) {
		return "", pm.ErrTenantRequired
	}
	if u.TenantID == platformIdentityScope {
		return "", domain.ErrValidation
	}
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	u.Email = normalized(u.Email)
	s, scopeErr := r.explicit(u.TenantID)
	if scopeErr != nil {
		return "", scopeErr
	}
	a, err := r.load(ctx, s, bson.M{"email": u.Email})
	if err == nil {
		err = r.mutate(ctx, s, bson.M{"_id": a.ID}, func(a *account) error {
			if a.User != nil {
				return domain.ErrConflict
			}
			a.User = &u
			a.Credential = c
			return nil
		})
		return u.ID, err
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return "", err
	}
	_, err = s.InsertOne(ctx, bson.M{"_id": uuid.NewString(), "email": u.Email, "version": int64(0), "user": u, "credential": c, "deleted": false})
	return u.ID, mapped(err)
}
func (r *Repository) ListUsers(ctx context.Context) ([]domain.User, error) {
	s, err := r.scope(ctx)
	if err != nil {
		return nil, err
	}
	cursor, err := s.Find(ctx, bson.M{"user": bson.M{"$ne": nil}, "deleted": bson.M{"$ne": true}})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			slog.Error("close identity cursor", "err", err)
		}
	}()
	out := []domain.User{}
	for cursor.Next(ctx) {
		var a account
		if err = cursor.Decode(&a); err != nil {
			return nil, err
		}
		out = append(out, *a.User)
	}
	return out, cursor.Err()
}
func (r *Repository) GetUser(ctx context.Context, id string) (domain.User, error) {
	s, err := r.scope(ctx)
	if err != nil {
		return domain.User{}, err
	}
	a, err := r.load(ctx, s, bson.M{"user.id": id})
	if err != nil {
		return domain.User{}, err
	}
	return *a.User, nil
}
func patchUser(a *account, u domain.User) {
	if u.Name != "" {
		a.User.Name = u.Name
	}
	if u.Role != "" {
		a.User.Role = u.Role
	}
	if u.Permissions != nil {
		a.User.Permissions = u.Permissions
	}
	if u.Status != "" {
		a.User.Status = u.Status
	}
}
func revoke(a *account, now time.Time, family string) {
	for i := range a.Refresh {
		if family == "" || a.Refresh[i].Family == family {
			if a.Refresh[i].Revoked == nil {
				a.Refresh[i].Revoked = &now
			}
		}
	}
}
func (r *Repository) update(ctx context.Context, id string, u domain.User, revokeSessions bool, events ...tenancy.CloudEvent) error {
	s, err := r.scope(ctx)
	if err != nil {
		return err
	}
	return r.mutate(ctx, s, bson.M{"user.id": id}, func(a *account) error {
		patchUser(a, u)
		if revokeSessions {
			revoke(a, r.now(), "")
		}
		return nil
	}, events...)
}
func (r *Repository) UpdateUser(ctx context.Context, id string, u domain.User) error {
	return r.update(ctx, id, u, false)
}
func (r *Repository) UpdateUserAndRevokeSessions(ctx context.Context, id string, u domain.User) error {
	return r.update(ctx, id, u, true)
}
func (r *Repository) UpdateUserWithRoleChange(ctx context.Context, id string, u domain.User, e ports.RoleChangeEvent) error {
	existing, err := r.GetUser(ctx, id)
	if err != nil {
		return err
	}
	if e.UserID != id || e.TenantID != existing.TenantID {
		return domain.ErrValidation
	}
	payload, err := json.Marshal(ports.RoleChangeEventData(e))
	if err != nil {
		return err
	}
	event := tenancy.CloudEvent{SpecVersion: "1.0",
		ID:       uuid.NewString(),
		Type:     "user.role_changed.v1",
		Source:   "identity-service",
		TenantID: tenantKey(e.TenantID),
		Time:     r.now().UTC().Format(time.RFC3339Nano),
		Data:     payload}
	return r.update(ctx, id, u, true, event)
}
func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	s, err := r.scope(ctx)
	if err != nil {
		return err
	}
	return r.mutate(ctx, s, bson.M{"user.id": id}, func(a *account) error {
		a.Deleted = true
		revoke(a, r.now(), "")
		a.Credential = domain.Credential{}
		a.MFA = nil
		return nil
	})
}
func (r *Repository) GetMFA(ctx context.Context, id string) (ports.MFARecord, bool, error) {
	s, scopeErr := r.privileged(ctx)
	if scopeErr != nil {
		return ports.MFARecord{}, false, scopeErr
	}
	a, err := r.load(ctx, s, bson.M{"user.id": id})
	if err != nil {
		return ports.MFARecord{}, false, err
	}
	if a.MFA == nil {
		return ports.MFARecord{}, false, nil
	}
	return *a.MFA, true, nil
}
func (r *Repository) SaveMFA(ctx context.Context, id string, secret []byte, counter int64) error {
	s, scopeErr := r.privileged(ctx)
	if scopeErr != nil {
		return scopeErr
	}
	return r.mutate(ctx, s, bson.M{"user.id": id}, func(a *account) error {
		if a.MFA != nil {
			return domain.ErrConflict
		}
		a.MFA = &ports.MFARecord{EncryptedSecret: secret, LastCounter: counter}
		return nil
	})
}
func (r *Repository) AdvanceMFACounter(ctx context.Context, id string, counter int64) error {
	s, scopeErr := r.privileged(ctx)
	if scopeErr != nil {
		return scopeErr
	}
	return r.mutate(ctx, s, bson.M{"user.id": id}, func(a *account) error {
		if a.MFA == nil || counter <= a.MFA.LastCounter {
			return domain.ErrInvalidCredentials
		}
		a.MFA.LastCounter = counter
		return nil
	})
}
func (r *Repository) SaveRefreshToken(ctx context.Context, id, hash, family string, expiry time.Time) error {
	if family == "" {
		return domain.ErrValidation
	}
	s, scopeErr := r.privileged(ctx)
	if scopeErr != nil {
		return scopeErr
	}
	return r.mutate(ctx, s, bson.M{"user.id": id}, func(a *account) error {
		for _, t := range a.Refresh {
			if t.Hash == hash {
				return domain.ErrConflict
			}
		}
		a.Refresh = append(a.Refresh, token{Hash: hash, Family: family, Expires: expiry})
		return nil
	})
}
func (r *Repository) FindRefreshToken(ctx context.Context, hash string) (string, error) {
	s, scopeErr := r.privileged(ctx)
	if scopeErr != nil {
		return "", scopeErr
	}
	a, err := r.load(ctx, s, bson.M{"refresh.hash": hash})
	if errors.Is(err, domain.ErrNotFound) {
		err = domain.ErrExpiredToken
	}
	if err != nil {
		return "", err
	}
	return a.User.ID, nil
}
func (r *Repository) RotateRefreshToken(ctx context.Context, oldHash, newHash string, expiry time.Time) (string, error) {
	s, scopeErr := r.privileged(ctx)
	if scopeErr != nil {
		return "", scopeErr
	}
	var id string
	var rejected bool
	err := r.mutate(ctx, s, bson.M{"refresh.hash": oldHash}, func(a *account) error {
		rejected = false
		for _, t := range a.Refresh {
			if t.Hash == newHash {
				return domain.ErrConflict
			}
		}
		for i := range a.Refresh {
			t := &a.Refresh[i]
			if t.Hash != oldHash {
				continue
			}
			now := r.now()
			id = a.User.ID
			if t.Revoked != nil || !now.Before(t.Expires) {
				revoke(a, now, t.Family)
				rejected = true
				return nil
			}
			t.Revoked = &now
			family := t.Family
			a.Refresh = append(a.Refresh, token{Hash: newHash, Family: family, Expires: expiry})
			return nil
		}
		return domain.ErrExpiredToken
	})
	if errors.Is(err, domain.ErrNotFound) || err == nil && rejected {
		return "", domain.ErrExpiredToken
	}
	return id, err
}
func (r *Repository) RevokeRefreshFamily(ctx context.Context, hash string) error {
	s, scopeErr := r.privileged(ctx)
	if scopeErr != nil {
		return scopeErr
	}
	err := r.mutate(ctx, s, bson.M{"refresh.hash": hash}, func(a *account) error {
		for _, t := range a.Refresh {
			if t.Hash == hash {
				revoke(a, r.now(), t.Family)
				return nil
			}
		}
		return domain.ErrExpiredToken
	})
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ErrExpiredToken
	}
	return err
}
func (r *Repository) RevokeUserSessions(ctx context.Context, id string) error {
	s, scopeErr := r.privileged(ctx)
	if scopeErr != nil {
		return scopeErr
	}
	return r.mutate(ctx, s, bson.M{"user.id": id}, func(a *account) error { revoke(a, r.now(), ""); return nil })
}
func (r *Repository) SavePasswordResetToken(ctx context.Context, tenant, id, hash string, expiry time.Time) error {
	s, scopeErr := r.explicit(tenant)
	if scopeErr != nil {
		return scopeErr
	}
	return r.mutate(ctx, s, bson.M{"user.id": id}, func(a *account) error {
		now := r.now()
		for i := range a.Resets {
			if a.Resets[i].Hash == hash {
				return domain.ErrConflict
			}
			if a.Resets[i].Used == nil && a.Resets[i].Revoked == nil {
				a.Resets[i].Revoked = &now
			}
		}
		a.Resets = append(a.Resets, token{Hash: hash, Expires: expiry})
		return nil
	})
}
func (r *Repository) ResetPasswordWithToken(ctx context.Context, hash, tenant string, c domain.Credential) error {
	s, scopeErr := r.explicit(tenant)
	if scopeErr != nil {
		return scopeErr
	}
	err := r.mutate(ctx, s, bson.M{"resets.hash": hash}, func(a *account) error {
		now := r.now()
		found := false
		for i := range a.Resets {
			t := &a.Resets[i]
			if t.Hash == hash {
				if t.Used != nil || t.Revoked != nil || !now.Before(t.Expires) {
					return domain.ErrExpiredToken
				}
				t.Used = &now
				found = true
			}
		}
		if !found {
			return domain.ErrExpiredToken
		}
		for i := range a.Resets {
			if a.Resets[i].Used == nil && a.Resets[i].Revoked == nil {
				a.Resets[i].Revoked = &now
			}
		}
		a.Credential = c
		revoke(a, now, "")
		return nil
	})
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ErrExpiredToken
	}
	return err
}
func (r *Repository) SaveInvite(ctx context.Context, tenant, email, role string, perms []string, hash string, _ *string, expiry time.Time) error {
	if tenant == platformIdentityScope {
		return domain.ErrValidation
	}
	s, scopeErr := r.explicit(tenant)
	if scopeErr != nil {
		return scopeErr
	}
	email = normalized(email)
	// Reserve the tenant/email aggregate before first credential installation;
	// invite acceptance and concurrent user creation then contend on the same CAS.
	_,
		err := s.UpsertOne(ctx,
		bson.M{"email": email,
			"deleted": false},
		bson.M{"$setOnInsert": bson.M{"_id": uuid.NewString(),
			"email":   email,
			"version": int64(0),
			"deleted": false}})
	if err != nil && !driver.IsDuplicateKeyError(err) {
		return err
	}
	return r.mutate(ctx, s, bson.M{"email": email}, func(a *account) error {
		now := r.now()
		for i := range a.Invites {
			t := &a.Invites[i].Token
			if t.Hash == hash {
				return domain.ErrConflict
			}
			if t.Used == nil && t.Revoked == nil {
				t.Revoked = &now
			}
		}
		a.Invites = append(a.Invites,
			invitation{Token: token{Hash: hash,
				Expires: expiry},
				Details: ports.InviteDetails{TenantID: tenant,
					Email:       email,
					Role:        role,
					Permissions: perms}})
		return nil
	})
}
func (r *Repository) InspectInvite(ctx context.Context, hash string) (ports.InviteDetails, error) {
	s, scopeErr := r.privileged(ctx)
	if scopeErr != nil {
		return ports.InviteDetails{}, scopeErr
	}
	a, err := r.load(ctx, s, bson.M{"invites.token.hash": hash})
	if errors.Is(err, domain.ErrNotFound) {
		err = domain.ErrExpiredToken
	}
	if err != nil {
		return ports.InviteDetails{}, err
	}
	for _, i := range a.Invites {
		if i.Token.Hash == hash && i.Token.Revoked == nil && r.now().Before(i.Token.Expires) {
			return i.Details, nil
		}
	}
	return ports.InviteDetails{}, domain.ErrExpiredToken
}
func (r *Repository) AcceptInviteWithCredential(ctx context.Context, hash, name, password string, c domain.Credential) (domain.User, error) {
	s, scopeErr := r.privileged(ctx)
	if scopeErr != nil {
		return domain.User{}, scopeErr
	}
	var user domain.User
	err := r.mutate(ctx, s, bson.M{"invites.token.hash": hash}, func(a *account) error {
		for i := range a.Invites {
			inv := &a.Invites[i]
			if inv.Token.Hash != hash {
				continue
			}
			now := r.now()
			if inv.Token.Revoked != nil || !now.Before(inv.Token.Expires) {
				return domain.ErrExpiredToken
			}
			d := inv.Details
			if a.User == nil {
				a.User = &domain.User{ID: uuid.NewString(),
					TenantID:    d.TenantID,
					Email:       d.Email,
					Name:        strings.TrimSpace(name),
					Role:        d.Role,
					Permissions: d.Permissions,
					Status:      domain.StatusActive}
				a.Credential = c
			} else {
				if a.User.Role != d.Role || a.User.Status != domain.StatusActive || !slices.Equal(a.User.Permissions, d.Permissions) {
					return domain.ErrExpiredToken
				}
				if len(a.Credential.Hash) == 0 {
					a.Credential = c
				} else if !a.Credential.Verify(password) {
					return domain.ErrExpiredToken
				}
			}
			if inv.Token.Used == nil {
				inv.Token.Used = &now
			}
			user = *a.User
			return nil
		}
		return domain.ErrExpiredToken
	})
	if errors.Is(err, domain.ErrNotFound) {
		err = domain.ErrExpiredToken
	}
	return user, err
}
