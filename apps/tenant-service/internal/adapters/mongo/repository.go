// Package mongo persists tenants, their feature flags and onboarding in MongoDB.
//
// It implements the same ports as the Postgres adapter and is selected by
// platform/store, so neither driver is privileged and switching is config.
//
// Two things here differ from a typical service. Tenant records are keyed by
// their own code rather than a tenant_id column, matching the RLS policies, so
// those collections declare that field. And the onboarding queue is
// platform-owned: a school applying to join has no tenant yet, which its
// PostgreSQL table records by being rls-exempt.
package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/auraedu/platform/auth"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/auraedu/tenant-service/internal/ports"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	TenantCollection       = "tenants"
	FeatureCollection      = "tenant_features"
	OnboardingCollection   = "onboarding_requests"
	CustomDomainCollection = "tenant_custom_domains"
	outboxLease            = 5 * time.Minute
	eventSource            = "tenant-service"
)

type Repository struct {
	store         *pmongo.Store
	tenants       *pmongo.Collection
	features      *pmongo.Collection
	onboarding    *pmongo.Collection
	customDomains *pmongo.Collection

	tenantOutbox     *pmongo.ClaimableOutbox
	featureOutbox    *pmongo.ClaimableOutbox
	onboardingOutbox *pmongo.ClaimableOutbox
}

var (
	_ ports.Repository                       = (*Repository)(nil)
	_ ports.OutboxRepository                 = (*Repository)(nil)
	_ ports.DurableOnboardingRepository      = (*Repository)(nil)
	_ ports.DurableTenantLifecycleRepository = (*Repository)(nil)
)

func NewRepository(store *pmongo.Store) *Repository {
	// The owner field mirrors each table's RLS column rather than tenant_id.
	tenants := store.Collection(TenantCollection).WithTenantField("code")
	features := store.Collection(FeatureCollection).WithTenantField("tenant_code")
	customDomains := store.Collection(CustomDomainCollection).WithTenantField("tenant_code")
	onboarding := store.Collection(OnboardingCollection)

	return &Repository{store: store,
		tenants: tenants, features: features, onboarding: onboarding, customDomains: customDomains,
		tenantOutbox:     pmongo.NewClaimableOutbox(tenants, outboxLease),
		featureOutbox:    pmongo.NewClaimableOutbox(features, outboxLease),
		onboardingOutbox: pmongo.NewClaimableOutbox(onboarding, outboxLease),
	}
}

func (*Repository) OnboardingEventsDurable()      {}
func (*Repository) TenantLifecycleEventsDurable() {}

// tenantCtx scopes a context to one tenant. The application calls these methods
// with the tenant in the argument rather than the context, exactly as it does
// for the Postgres adapter, which derives app.tenant_id the same way.
func tenantCtx(ctx context.Context, code string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: code})
}

func notFound(err error, code string) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return fmt.Errorf("%w: %s", domain.ErrNotFound, code)
	}
	return err
}

func tenantEvent(code, eventType string, payload map[string]any) (tenancy.CloudEvent, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return tenancy.CloudEvent{}, fmt.Errorf("encode %s outbox event: %w", eventType, err)
	}
	return tenancy.CloudEvent{
		SpecVersion: "1.0", Type: eventType, Source: eventSource,
		ID: uuid.NewString(), Time: time.Now().UTC().Format(time.RFC3339),
		TenantID: code, Data: encoded,
	}, nil
}

// ---- tenants -------------------------------------------------------------

type tenantDoc struct {
	Code                   string    `bson:"_id"`
	TenantCode             string    `bson:"code"`
	Name                   string    `bson:"name"`
	Short                  string    `bson:"short,omitempty"`
	Status                 string    `bson:"status"`
	Domain                 string    `bson:"domain,omitempty"`
	Plan                   string    `bson:"plan"`
	BrandPrimary           string    `bson:"brand_primary,omitempty"`
	BrandSecondary         string    `bson:"brand_secondary,omitempty"`
	LogoURL                string    `bson:"logo_url,omitempty"`
	Locale                 string    `bson:"locale,omitempty"`
	Timezone               string    `bson:"timezone,omitempty"`
	DateFormat             string    `bson:"date_format,omitempty"`
	AcademicYearStartMonth int       `bson:"academic_year_start_month,omitempty"`
	PrimaryContactEmail    string    `bson:"primary_contact_email,omitempty"`
	CreatedAt              time.Time `bson:"created_at"`
	UpdatedAt              time.Time `bson:"updated_at"`
}

func (d tenantDoc) toDomain() domain.Tenant {
	t := domain.Tenant{
		Code: d.TenantCode, Name: d.Name, Short: d.Short, Status: d.Status,
		Domain: d.Domain, Plan: d.Plan,
	}
	t.Branding.Brand.Primary = d.BrandPrimary
	t.Branding.Brand.Secondary = d.BrandSecondary
	t.Branding.LogoURL = d.LogoURL
	return t
}

func (d tenantDoc) settings() domain.Settings {
	return domain.Settings{
		Locale: d.Locale, Timezone: d.Timezone, DateFormat: d.DateFormat,
		AcademicYearStartMonth: d.AcademicYearStartMonth, PrimaryContactEmail: d.PrimaryContactEmail,
	}
}

func tenantFields(t domain.Tenant) bson.M {
	return bson.M{
		"name": t.Name, "short": t.Short, "status": t.Status, "domain": t.Domain,
		"plan": t.Plan, "brand_primary": t.Branding.Brand.Primary,
		"brand_secondary": t.Branding.Brand.Secondary, "logo_url": t.Branding.LogoURL,
		"updated_at": time.Now().UTC(),
	}
}

// ListTenants is a platform-wide read with no single tenant scope, so it runs as
// a platform admin — the same authority the Postgres adapter sets for it.
func (r *Repository) ListTenants(ctx context.Context) ([]domain.Tenant, error) {
	scope, err := r.tenants.Scope(auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true}))
	if err != nil {
		return nil, err
	}
	cur, err := scope.Find(ctx, bson.M{"deleted_at": bson.M{"$exists": false}},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []domain.Tenant
	for cur.Next(ctx) {
		var doc tenantDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	return out, cur.Err()
}

func (r *Repository) GetTenant(ctx context.Context, code string) (domain.Tenant, error) {
	doc, err := r.tenantDoc(ctx, code)
	if err != nil {
		return domain.Tenant{}, err
	}
	return doc.toDomain(), nil
}

func (r *Repository) tenantDoc(ctx context.Context, code string) (tenantDoc, error) {
	scope, err := r.tenants.ScopeTo(code)
	if err != nil {
		return tenantDoc{}, err
	}
	var doc tenantDoc
	if err := scope.FindOne(ctx, bson.M{"_id": code, "deleted_at": bson.M{"$exists": false}}).Decode(&doc); err != nil {
		return tenantDoc{}, notFound(err, code)
	}
	return doc, nil
}

func (r *Repository) CreateTenant(ctx context.Context, t domain.Tenant) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		scope, err := r.tenants.ScopeTo(t.Code)
		if err != nil {
			return err
		}
		event, err := tenantEvent(t.Code, "tenant.created.v1", map[string]any{
			"tenant_code": t.Code, "name": t.Name, "plan": t.Plan, "status": t.Status,
		})
		if err != nil {
			return err
		}
		doc := tenantFields(t)
		doc["_id"] = t.Code
		doc["created_at"] = time.Now().UTC()
		if _, err := scope.InsertWithEvents(ctx, doc, event); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return domain.ErrConflict
			}
			return fmt.Errorf("create tenant: %w", err)
		}
		return r.seedFeatures(ctx, t)
	})
}

func (r *Repository) UpdateTenant(ctx context.Context, code string, upd domain.TenantUpdate) (domain.Tenant, error) {
	current, err := r.GetTenant(ctx, code)
	if err != nil {
		return domain.Tenant{}, err
	}
	next := current.ApplyUpdate(upd)
	if err := next.Validate(); err != nil {
		return domain.Tenant{}, err
	}

	scope, err := r.tenants.ScopeTo(code)
	if err != nil {
		return domain.Tenant{}, err
	}
	event, err := tenantEvent(code, "tenant.updated.v1", map[string]any{
		"tenant_code": code, "name": next.Name, "plan": next.Plan, "status": next.Status,
	})
	if err != nil {
		return domain.Tenant{}, err
	}
	res, err := scope.UpdateWithEvents(ctx, bson.M{"_id": code, "deleted_at": bson.M{"$exists": false}}, bson.M{"$set": tenantFields(next)}, event)
	if err != nil {
		return domain.Tenant{}, fmt.Errorf("update tenant: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.Tenant{}, fmt.Errorf("%w: %s", domain.ErrNotFound, code)
	}
	return next, nil
}

// DeleteTenant tombstones the record so its deletion event survives to be
// published, then PurgeSettled clears it once delivered.
func (r *Repository) DeleteTenant(ctx context.Context, code string) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		scope, err := r.tenants.ScopeTo(code)
		if err != nil {
			return err
		}
		event, err := tenantEvent(code, "tenant.deleted.v1", map[string]any{"tenant_code": code})
		if err != nil {
			return err
		}
		res, err := scope.UpdateWithEvents(ctx,
			bson.M{"_id": code, "deleted_at": bson.M{"$exists": false}},
			bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}}, event)
		if err != nil {
			return fmt.Errorf("delete tenant: %w", err)
		}
		if res.MatchedCount == 0 {
			return fmt.Errorf("%w: %s", domain.ErrNotFound, code)
		}
		for _, collection := range []*pmongo.Collection{r.features, r.customDomains} {
			owned, err := collection.ScopeTo(code)
			if err != nil {
				return err
			}
			if _, err = owned.UpdateMany(ctx, bson.M{}, bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}}); err != nil {
				return err
			}
		}
		return nil
	})
}

// ResolveTenant maps a host or subdomain to an active tenant. It is a
// platform-wide lookup by design: the caller has no tenant yet, which is what it
// is trying to establish, and the Postgres adapter runs it as a platform admin
// for the same reason.
func (r *Repository) ResolveTenant(ctx context.Context, domainHost, subdomain string) (domain.Tenant, error) {
	or := bson.A{}
	if domainHost != "" {
		or = append(or, bson.M{"domain": bson.M{"$regex": "^" + regexp.QuoteMeta(domainHost) + "$", "$options": "i"}})
	}
	if subdomain != "" {
		or = append(or, bson.M{"_id": strings.ToLower(subdomain)})
	}
	if len(or) == 0 {
		return domain.Tenant{}, fmt.Errorf("%w: %s", domain.ErrNotFound, domainHost+subdomain)
	}

	var doc tenantDoc
	scope, err := r.tenants.Scope(auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true}))
	if err != nil {
		return domain.Tenant{}, err
	}
	err = scope.FindOne(ctx, bson.M{
		"status": "active", "deleted_at": bson.M{"$exists": false}, "$or": or,
	}).Decode(&doc)
	if err != nil {
		return domain.Tenant{}, notFound(err, domainHost+subdomain)
	}
	return doc.toDomain(), nil
}

// ---- settings ------------------------------------------------------------

func (r *Repository) Settings(ctx context.Context, code string) (domain.Settings, error) {
	doc, err := r.tenantDoc(ctx, code)
	if err != nil {
		return domain.Settings{}, err
	}
	return doc.settings(), nil
}

func (r *Repository) UpdateSettings(ctx context.Context, code string, s domain.Settings) error {
	scope, err := r.tenants.ScopeTo(code)
	if err != nil {
		return err
	}
	res, err := scope.UpdateOne(ctx, bson.M{"_id": code, "deleted_at": bson.M{"$exists": false}}, bson.M{"$set": bson.M{
		"locale": s.Locale, "timezone": s.Timezone, "date_format": s.DateFormat,
		"academic_year_start_month": s.AcademicYearStartMonth,
		"primary_contact_email":     s.PrimaryContactEmail,
		"updated_at":                time.Now().UTC(),
	}})
	if err != nil {
		return fmt.Errorf("update settings: %w", err)
	}
	if res.MatchedCount != 1 {
		return fmt.Errorf("%w: %s", domain.ErrNotFound, code)
	}
	return nil
}

// ---- features ------------------------------------------------------------

// A tenant's flags live in ONE document rather than one per flag.
//
// PostgreSQL seeds all forty-odd catalogue rows inside the transaction that
// creates the tenant, and changes one row alongside its event. Flags are stored in
// a single document within the tenant provisioning transaction: seeding is one atomic write, and setting a flag is one
// atomic write that carries its own event.
type featureSetDoc struct {
	TenantCode string                `bson:"_id"`
	Owner      string                `bson:"tenant_code"`
	Flags      map[string]featureDoc `bson:"flags"`
}

type featureDoc struct {
	IsEnabled         bool   `bson:"is_enabled"`
	Reason            string `bson:"reason,omitempty"`
	RolloutPercentage *int   `bson:"rollout_percentage,omitempty"`
	RolloutUpdatedBy  string `bson:"rollout_updated_by,omitempty"`
	RolloutReason     string `bson:"rollout_reason,omitempty"`
}

// seedFeatures writes the catalogue with each flag defaulted by the tenant's
// plan, mirroring what createTenant does in the same transaction on PostgreSQL.
func (r *Repository) seedFeatures(ctx context.Context, t domain.Tenant) error {
	scope, err := r.features.ScopeTo(t.Code)
	if err != nil {
		return err
	}
	flags := bson.M{}
	for _, f := range domain.FeatureCatalog() {
		flags[f.Key] = bson.M{"is_enabled": domain.PlanAllows(t.Plan, f.PlanRequired)}
	}
	// A reused code receives fresh defaults; keep old pending outbox events intact.
	restored, err := scope.UpdateOne(ctx, bson.M{"_id": t.Code, "deleted_at": bson.M{"$exists": true}},
		bson.M{"$set": bson.M{"flags": flags}, "$unset": bson.M{"deleted_at": ""}})
	if err != nil {
		return fmt.Errorf("reseed features: %w", err)
	}
	if restored.MatchedCount == 1 {
		return nil
	}
	if _, err := scope.InsertOne(ctx, bson.M{"_id": t.Code, "flags": flags}); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrConflict
		}
		return fmt.Errorf("seed features: %w", err)
	}
	return nil
}

// Features returns the whole catalogue with this tenant's flags applied. A
// tenant with no flag document does not exist, or is not visible to this scope —
// the same conclusion the PostgreSQL adapter draws from finding no rows.
func (r *Repository) Features(ctx context.Context, code string) ([]domain.FeatureFlag, error) {
	if _, err := r.GetTenant(ctx, code); err != nil {
		return nil, err
	}
	scope, err := r.features.ScopeTo(code)
	if err != nil {
		return nil, err
	}
	var doc featureSetDoc
	if err := scope.FindOne(ctx, bson.M{"_id": code, "deleted_at": bson.M{"$exists": false}}).Decode(&doc); err != nil {
		return nil, notFound(err, code)
	}

	catalog := domain.FeatureCatalog()
	out := make([]domain.FeatureFlag, 0, len(catalog))
	for _, f := range catalog {
		flag := domain.FeatureFlag{Key: f.Key, PlanRequired: f.PlanRequired}
		if stored, ok := doc.Flags[f.Key]; ok {
			flag.Enabled = stored.IsEnabled
			if stored.RolloutPercentage != nil {
				flag.Rollout = &domain.RolloutConfig{
					Percentage: *stored.RolloutPercentage,
					UpdatedBy:  stored.RolloutUpdatedBy,
					Reason:     stored.RolloutReason,
				}
			}
		}
		out = append(out, flag)
	}
	return out, nil
}

func (r *Repository) SetFeature(ctx context.Context, code, key string, enabled bool, reason string) (domain.FeatureFlag, error) {
	plan, known := domain.FeaturePlan(key)
	if !known {
		return domain.FeatureFlag{}, domain.ErrValidation
	}
	scope, err := r.features.ScopeTo(code)
	if err != nil {
		return domain.FeatureFlag{}, err
	}
	eventType := "tenant.feature_disabled.v1"
	if enabled {
		eventType = "tenant.feature_enabled.v1"
	}
	event, err := tenantEvent(code, eventType, map[string]any{
		"feature_key": key, "is_enabled": enabled, "plan": plan,
	})
	if err != nil {
		return domain.FeatureFlag{}, err
	}

	// The flag and the event announcing it land together, in one atomic
	// single-document write.
	res, err := scope.UpdateWithEvents(ctx, bson.M{"_id": code, "deleted_at": bson.M{"$exists": false}}, bson.M{
		"$set": bson.M{
			"flags." + key + ".is_enabled": enabled,
			"flags." + key + ".reason":     reason,
			"updated_at":                   time.Now().UTC(),
		},
	}, event)
	if err != nil {
		return domain.FeatureFlag{}, fmt.Errorf("upsert feature: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.FeatureFlag{}, fmt.Errorf("%w: %s", domain.ErrNotFound, code)
	}
	return domain.FeatureFlag{Key: key, Enabled: enabled, PlanRequired: plan}, nil
}

// ---- onboarding ----------------------------------------------------------
//
// The onboarding queue is platform-owned: a school applying to join has no
// tenant yet, which its PostgreSQL table records by being rls-exempt.

type onboardingDoc struct {
	ID                   string     `bson:"_id"`
	SchoolName           string     `bson:"school_name"`
	AdministratorName    string     `bson:"administrator_name"`
	Email                string     `bson:"email"`
	Phone                *string    `bson:"phone,omitempty"`
	CountryCode          string     `bson:"country_code"`
	Plan                 string     `bson:"plan"`
	Priorities           *string    `bson:"priorities,omitempty"`
	PrivacyNoticeVersion string     `bson:"privacy_notice_version"`
	Status               string     `bson:"status"`
	TenantCode           *string    `bson:"tenant_code,omitempty"`
	DecisionReason       *string    `bson:"decision_reason,omitempty"`
	DecidedBy            *string    `bson:"decided_by,omitempty"`
	DecidedAt            *time.Time `bson:"decided_at,omitempty"`
	IdempotencyHash      string     `bson:"idempotency_hash"`
	PayloadHash          string     `bson:"payload_hash"`
	EmailFingerprint     string     `bson:"email_fingerprint"`
	SubmittedAt          time.Time  `bson:"submitted_at"`
}

func (d onboardingDoc) toDomain() domain.OnboardingRequest {
	return domain.OnboardingRequest{
		ID: d.ID, SchoolName: d.SchoolName, AdministratorName: d.AdministratorName,
		Email: d.Email, Phone: d.Phone, CountryCode: d.CountryCode, Plan: d.Plan,
		Priorities: d.Priorities, PrivacyNoticeVersion: d.PrivacyNoticeVersion,
		Status: d.Status, TenantCode: d.TenantCode, DecisionReason: d.DecisionReason,
		DecidedAt: d.DecidedAt, SubmittedAt: d.SubmittedAt,
	}
}

// SubmitOnboarding is idempotent on idempotency_hash. A replay returns the
// stored request; a replay whose payload differs is a conflict, because the same
// key must not describe two different submissions.
func (r *Repository) SubmitOnboarding(
	ctx context.Context,
	request *domain.OnboardingRequest,
	idempotencyHash string,
	payloadHash string,
	emailFingerprint string,
) (*domain.OnboardingRequest, bool, error) {
	queue := r.onboarding.PlatformOwned()
	doc := bson.M{
		"_id": request.ID, "school_name": request.SchoolName,
		"administrator_name": request.AdministratorName, "email": request.Email,
		"phone": request.Phone, "country_code": request.CountryCode, "plan": request.Plan,
		"priorities": request.Priorities, "privacy_notice_version": request.PrivacyNoticeVersion,
		"status": request.Status, "idempotency_hash": idempotencyHash,
		"payload_hash": payloadHash, "email_fingerprint": emailFingerprint,
		"submitted_at": request.SubmittedAt,
	}
	_, err := queue.InsertOne(ctx, doc)
	if err == nil {
		return request, true, nil
	}
	if !mongo.IsDuplicateKeyError(err) {
		return nil, false, fmt.Errorf("onboarding: submit: %w", err)
	}

	var existing onboardingDoc
	if err := queue.FindOne(ctx, bson.M{"idempotency_hash": idempotencyHash}).Decode(&existing); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, false, domain.ErrConflict
		}
		return nil, false, fmt.Errorf("onboarding: find replay: %w", err)
	}
	if existing.PayloadHash != "" && existing.PayloadHash != payloadHash {
		return nil, false, domain.ErrConflict
	}
	found := existing.toDomain()
	return &found, false, nil
}

func (r *Repository) ListOnboarding(ctx context.Context, limit int, cursor, status string) ([]domain.OnboardingRequest, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	query := bson.M{}
	if status != "" {
		query["status"] = status
	}
	if cursor != "" {
		query["_id"] = bson.M{"$gt": cursor}
	}
	cur, err := r.onboarding.PlatformOwned().Find(ctx, query,
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("onboarding: list: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []domain.OnboardingRequest
	for cur.Next(ctx) {
		var doc onboardingDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("onboarding: scan: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, "", err
	}
	var next string
	if len(out) == limit && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func (r *Repository) GetOnboarding(ctx context.Context, requestID string) (domain.OnboardingRequest, error) {
	var doc onboardingDoc
	if err := r.onboarding.PlatformOwned().FindOne(ctx, bson.M{"_id": requestID}).Decode(&doc); err != nil {
		return domain.OnboardingRequest{}, notFound(err, requestID)
	}
	return doc.toDomain(), nil
}

// ApproveOnboarding provisions the tenant, feature defaults, contact and both downstream
// events in the same transaction as the intake decision. Existing tenants conflict.
func (r *Repository) ApproveOnboarding(ctx context.Context, requestID string, tenant domain.Tenant, decidedBy string) (domain.OnboardingRequest, error) {
	var approved domain.OnboardingRequest
	err := r.store.WithTransaction(ctx, func(ctx context.Context) error {
		current, err := r.GetOnboarding(ctx, requestID)
		if err != nil {
			return err
		}
		if current.Status != domain.OnboardingPending {
			return domain.ErrConflict
		}
		if err = r.CreateTenant(ctx, tenant); err != nil {
			return err
		}
		event,
			err := tenantEvent(tenant.Code,
			"tenant.onboarding_approved.v1",
			map[string]any{"request_id": requestID,
				"tenant_code": tenant.Code,
				"plan":        tenant.Plan})
		if err != nil {
			return err
		}
		scope, err := r.tenants.ScopeTo(tenant.Code)
		if err != nil {
			return err
		}
		res,
			err := scope.UpdateWithEvents(ctx,
			bson.M{"_id": tenant.Code,
				"deleted_at": bson.M{"$exists": false}},
			bson.M{"$set": bson.M{"primary_contact_email": current.Email}},
			event)
		if err != nil {
			return err
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
		now := time.Now().UTC()
		res,
			err = r.onboarding.PlatformOwned().UpdateOne(ctx,
			bson.M{"_id": requestID,
				"status": domain.OnboardingPending},
			bson.M{"$set": bson.M{"status": domain.OnboardingApproved,
				"tenant_code": tenant.Code,
				"decided_by":  decidedBy,
				"decided_at":  now}})
		if err != nil {
			return err
		}
		if res.MatchedCount != 1 {
			return domain.ErrConflict
		}
		approved = current
		approved.Status = domain.OnboardingApproved
		approved.TenantCode = &tenant.Code
		approved.DecidedAt = &now
		return nil
	})
	if err != nil {
		return domain.OnboardingRequest{}, err
	}
	return approved, nil
}

func (r *Repository) RejectOnboarding(ctx context.Context, requestID, reason, decidedBy string) (domain.OnboardingRequest, error) {
	decidedAt := time.Now().UTC()
	res, err := r.onboarding.PlatformOwned().UpdateOne(ctx,
		bson.M{"_id": requestID, "status": "pending_review"},
		bson.M{"$set": bson.M{
			"status": "rejected", "decision_reason": reason,
			"decided_by": decidedBy, "decided_at": decidedAt,
		}})
	if err != nil {
		return domain.OnboardingRequest{}, fmt.Errorf("onboarding: reject: %w", err)
	}
	if res.MatchedCount != 1 {
		// Either the request does not exist or it was already decided.
		if _, err := r.GetOnboarding(ctx, requestID); err != nil {
			return domain.OnboardingRequest{}, err
		}
		return domain.OnboardingRequest{}, domain.ErrConflict
	}
	return r.GetOnboarding(ctx, requestID)
}

// ActivateOnboardingTenant flips an onboarding tenant to active and records
// exactly one activation event. A repeated call is a successful no-op: the
// filter requires the onboarding status, so the second caller matches nothing
// and queues no second event.
func (r *Repository) ActivateOnboardingTenant(ctx context.Context, code string) (bool, error) {
	doc, err := r.tenantDoc(ctx, code)
	if err != nil {
		return false, err
	}
	if doc.Status == "active" {
		return false, nil
	}
	if doc.Status != "onboarding" {
		return false, domain.ErrConflict
	}

	scope, err := r.tenants.ScopeTo(code)
	if err != nil {
		return false, err
	}
	event, err := tenantEvent(code, "tenant.activated.v1", map[string]any{
		"tenant_code": code, "status": "active",
	})
	if err != nil {
		return false, err
	}
	res, err := scope.UpdateWithEvents(ctx,
		bson.M{"_id": code, "status": "onboarding", "deleted_at": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"status": "active", "updated_at": time.Now().UTC()}}, event)
	if err != nil {
		return false, fmt.Errorf("activate tenant: %w", err)
	}
	return res.MatchedCount == 1, nil
}

// ---- custom domains ------------------------------------------------------

type customDomainDoc struct {
	TenantCode        string     `bson:"_id"`
	Owner             string     `bson:"tenant_code"`
	Hostname          string     `bson:"hostname"`
	Status            string     `bson:"status"`
	TXTRecordName     string     `bson:"txt_record_name"`
	ChallengeHash     string     `bson:"challenge_hash"`
	VerifiedAt        *time.Time `bson:"verified_at,omitempty"`
	ActivatedAt       *time.Time `bson:"activated_at,omitempty"`
	DeactivatedAt     *time.Time `bson:"deactivated_at,omitempty"`
	ProviderReference string     `bson:"provider_reference,omitempty"`
}

func (d customDomainDoc) toDomain() domain.CustomDomain {
	return domain.CustomDomain{
		TenantCode: d.Owner, Hostname: d.Hostname, Status: d.Status,
		TXTRecordName: d.TXTRecordName, VerifiedAt: d.VerifiedAt,
		ActivatedAt: d.ActivatedAt, DeactivatedAt: d.DeactivatedAt,
		ProviderReference: d.ProviderReference,
	}
}

func (r *Repository) requestCustomDomain(ctx context.Context, registration domain.CustomDomain, challengeHash string) (domain.CustomDomain, error) {
	scope, err := r.customDomains.ScopeTo(registration.TenantCode)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	if _, err := scope.UpsertOne(ctx, bson.M{"_id": registration.TenantCode}, bson.M{
		"$set": bson.M{
			"hostname": registration.Hostname, "status": registration.Status,
			"txt_record_name": registration.TXTRecordName, "challenge_hash": challengeHash,
			"verified_at": nil, "activated_at": nil, "deactivated_at": nil,
			"provider_reference": "", "updated_at": time.Now().UTC(),
		},
	}); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// hostname is globally unique: another tenant already claimed it.
			return domain.CustomDomain{}, domain.ErrConflict
		}
		return domain.CustomDomain{}, fmt.Errorf("custom domain: request: %w", err)
	}
	return r.customDomain(ctx, registration.TenantCode)
}

func (r *Repository) customDomain(ctx context.Context, code string) (domain.CustomDomain, error) {
	scope, err := r.customDomains.ScopeTo(code)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	var doc customDomainDoc
	if err := scope.FindOne(ctx, bson.M{"_id": code, "deleted_at": bson.M{"$exists": false}}).Decode(&doc); err != nil {
		return domain.CustomDomain{}, notFound(err, code)
	}
	return doc.toDomain(), nil
}

// GetCustomDomain also returns the stored challenge hash, which the caller
// compares against a freshly computed one. The plaintext token is never stored.
func (r *Repository) GetCustomDomain(ctx context.Context, code string) (domain.CustomDomain, string, error) {
	scope, err := r.customDomains.ScopeTo(code)
	if err != nil {
		return domain.CustomDomain{}, "", err
	}
	var doc customDomainDoc
	if err := scope.FindOne(ctx, bson.M{"_id": code, "deleted_at": bson.M{"$exists": false}}).Decode(&doc); err != nil {
		return domain.CustomDomain{}, "", notFound(err, code)
	}
	return doc.toDomain(), doc.ChallengeHash, nil
}

func (r *Repository) markCustomDomain(ctx context.Context, code string, filter, set bson.M) (domain.CustomDomain, error) {
	scope, err := r.customDomains.ScopeTo(code)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	query := bson.M{"_id": code, "deleted_at": bson.M{"$exists": false}}
	for k, v := range filter {
		query[k] = v
	}
	set["updated_at"] = time.Now().UTC()
	res, err := scope.UpdateOne(ctx, query, bson.M{"$set": set})
	if err != nil {
		return domain.CustomDomain{}, fmt.Errorf("custom domain: update: %w", err)
	}
	if res.MatchedCount != 1 {
		// Present but in the wrong state is a conflict, absent is not found.
		if _, err := r.customDomain(ctx, code); err != nil {
			return domain.CustomDomain{}, err
		}
		return domain.CustomDomain{}, domain.ErrConflict
	}
	return r.customDomain(ctx, code)
}

func (r *Repository) MarkCustomDomainVerified(ctx context.Context, code string, verifiedAt time.Time) (domain.CustomDomain, error) {
	return r.markCustomDomain(ctx, code,
		bson.M{"status": "pending_dns"},
		bson.M{"status": "verified", "verified_at": verifiedAt})
}

func (r *Repository) ActivateCustomDomain(ctx context.Context, code, providerReference string, at time.Time) (domain.CustomDomain, error) {
	return r.transitionDomain(ctx, code, providerReference, at, true)
}
func (r *Repository) DeactivateCustomDomain(ctx context.Context, code, providerReference string, at time.Time) (domain.CustomDomain, error) {
	return r.transitionDomain(ctx, code, providerReference, at, false)
}
func (r *Repository) transitionDomain(ctx context.Context, code, reference string, at time.Time, active bool) (domain.CustomDomain, error) {
	var result domain.CustomDomain
	err := r.store.WithTransaction(ctx, func(ctx context.Context) error {
		from, to, kind := "active", "inactive", "tenant.custom_domain_deactivated.v1"
		set := bson.M{"deactivated_at": at, "provider_reference": reference}
		if active {
			from, to, kind = "verified", "active", "tenant.custom_domain_activated.v1"
			set = bson.M{"activated_at": at, "provider_reference": reference}
		}
		set["status"] = to
		var err error
		result, err = r.markCustomDomain(ctx, code, bson.M{"status": from}, set)
		if err != nil {
			return err
		}
		scope, err := r.tenants.ScopeTo(code)
		if err != nil {
			return err
		}
		query := bson.M{"_id": code, "deleted_at": bson.M{"$exists": false}}
		nextDomain := ""
		if active {
			nextDomain = result.Hostname
		} else {
			current, err := r.GetTenant(ctx, code)
			if err != nil {
				return err
			}
			if current.Domain != result.Hostname {
				nextDomain = current.Domain
			}
		}
		event, err := tenantEvent(code, kind, map[string]any{"tenant_code": code, "hostname": result.Hostname})
		if err != nil {
			return err
		}
		res, err := scope.UpdateWithEvents(ctx, query, bson.M{"$set": bson.M{"domain": nextDomain, "updated_at": time.Now().UTC()}}, event)
		if err != nil {
			return err
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return domain.CustomDomain{}, err
	}
	return result, nil
}

// ---- outbox --------------------------------------------------------------
//
// Events live inside the tenant or feature document they describe, so the domain
// change and its event are one atomic write. Claiming therefore drains both
// collections.

func claimedEvents(events []pmongo.ClaimedEvent) []ports.OutboxEvent {
	out := make([]ports.OutboxEvent, 0, len(events))
	for _, c := range events {
		out = append(out, ports.OutboxEvent{
			ID: c.Event.ID, TenantID: c.Event.TenantID, EventType: c.Event.Type,
			Payload: json.RawMessage(c.Event.Data), CreatedAt: time.Now().UTC(),
		})
	}
	return out
}

func (r *Repository) ClaimPending(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	fromTenants, err := r.tenantOutbox.Claim(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("tenant: claim outbox: %w", err)
	}
	out := claimedEvents(fromTenants)
	if len(out) >= limit {
		return out, nil
	}
	fromFeatures, err := r.featureOutbox.Claim(ctx, limit-len(out))
	if err != nil {
		return nil, fmt.Errorf("tenant: claim feature outbox: %w", err)
	}
	return append(out, claimedEvents(fromFeatures)...), nil
}

// MarkPublished acknowledges an event from either collection. The port carries
// only the event id, and acknowledging one that is already gone is normal for an
// at-least-once outbox.
func (r *Repository) MarkPublished(ctx context.Context, id string) error {
	if err := r.tenantOutbox.MarkPublishedByEventID(ctx, id); err != nil {
		return fmt.Errorf("tenant: mark published: %w", err)
	}
	if err := r.featureOutbox.MarkPublishedByEventID(ctx, id); err != nil {
		return fmt.Errorf("tenant: mark feature published: %w", err)
	}
	// A deleted tenant is held as a tombstone until its event is delivered.
	if _, err := r.tenantOutbox.PurgeSettled(ctx, bson.M{"deleted_at": bson.M{"$exists": true}}); err != nil {
		return err
	}
	return nil
}

func (r *Repository) MarkFailed(ctx context.Context, id, message string) error {
	if err := r.tenantOutbox.MarkFailedByEventID(ctx, id, message); err != nil {
		return fmt.Errorf("tenant: mark failed: %w", err)
	}
	if err := r.featureOutbox.MarkFailedByEventID(ctx, id, message); err != nil {
		return fmt.Errorf("tenant: mark feature failed: %w", err)
	}
	return nil
}

// EnsureIndexes replaces the CREATE INDEX and UNIQUE statements in the
// PostgreSQL migrations.
func EnsureIndexes(ctx context.Context, store *pmongo.Store) error {
	specs := map[string][]mongo.IndexModel{
		TenantCollection: {
			{Keys: bson.D{{Key: "created_at", Value: 1}}},
			{Keys: bson.D{{Key: "code", Value: 1}}},
		},
		FeatureCollection: {
			{Keys: bson.D{{Key: "tenant_code", Value: 1}}},
		},
		OnboardingCollection: {
			{Keys: bson.D{{Key: "email_fingerprint",
				Value: 1}},
				Options: options.Index().SetName("onboarding_pending_email_unique").SetUnique(true).SetPartialFilterExpression(bson.M{"status": domain.OnboardingPending})},

			{Keys: bson.D{{Key: "idempotency_hash", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "status", Value: 1}, {Key: "_id", Value: 1}}},
		},
		CustomDomainCollection: {
			{Keys: bson.D{{Key: "hostname", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
	}
	for name, models := range specs {
		if _, err := store.Database().Collection(name).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("tenant: ensure indexes on %s: %w", name, err)
		}
	}
	return nil
}

func (r *Repository) RequestCustomDomain(ctx context.Context, registration domain.CustomDomain, challengeHash string) (domain.CustomDomain, error) {
	var result domain.CustomDomain
	err := r.store.WithTransaction(ctx, func(ctx context.Context) error {
		scope, err := r.tenants.ScopeTo(registration.TenantCode)
		if err != nil {
			return err
		}
		res,
			err := scope.UpdateOne(ctx,
			bson.M{"_id": registration.TenantCode,
				"deleted_at": bson.M{"$exists": false}},
			bson.M{"$inc": bson.M{"_reference_version": 1}})
		if err != nil {
			return err
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
		result, err = r.requestCustomDomain(ctx, registration, challengeHash)
		return err
	})
	if err != nil {
		return domain.CustomDomain{}, err
	}
	return result, nil
}
