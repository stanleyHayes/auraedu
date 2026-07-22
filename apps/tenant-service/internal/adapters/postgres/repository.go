// Package postgres is a PostgreSQL+RLS implementation of ports.Repository.
// It uses platform/db so app.tenant_id is set for every transaction, enforcing
// row-level isolation keyed on tenant code.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/auraedu/tenant-service/internal/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Repository implements ports.Repository against PostgreSQL.
type Repository struct {
	db *db.DB
}

var _ ports.Repository = (*Repository)(nil)

// NewRepository returns a Postgres-backed tenant repository.
func NewRepository(d *db.DB) *Repository {
	return &Repository{db: d}
}

func withTenant(ctx context.Context, code string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: code})
}

func tenantNotFound(err error, code string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: %s", domain.ErrNotFound, code)
	}
	return err
}

// ListTenants returns all tenants ordered by creation time. It runs directly on
// the pool because this is a platform-wide operation with no single tenant scope.
func (r *Repository) ListTenants(ctx context.Context) ([]domain.Tenant, error) {
	var out []domain.Tenant
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tenants: begin: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := db.SetPlatformAdmin(ctx, tx); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT code, name, COALESCE(short, ''), status, COALESCE(domain, ''), plan,
		       COALESCE(brand_primary, ''), COALESCE(brand_secondary, ''), COALESCE(logo_url, '')
		FROM tenants
		ORDER BY created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var tenant domain.Tenant
		if err := rows.Scan(
			&tenant.Code, &tenant.Name, &tenant.Short, &tenant.Status, &tenant.Domain, &tenant.Plan,
			&tenant.Branding.Brand.Primary, &tenant.Branding.Brand.Secondary, &tenant.Branding.LogoURL,
		); err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}
		out = append(out, tenant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenants: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("list tenants: commit: %w", err)
	}
	return out, nil
}

// GetTenant returns a tenant by code.
func (r *Repository) GetTenant(ctx context.Context, code string) (domain.Tenant, error) {
	var t domain.Tenant
	err := r.db.WithTx(withTenant(ctx, code), func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT code, name, COALESCE(short, ''), status, COALESCE(domain, ''), plan,
			       COALESCE(brand_primary, ''), COALESCE(brand_secondary, ''), COALESCE(logo_url, '')
			FROM tenants
			WHERE code = $1
		`, code).Scan(
			&t.Code, &t.Name, &t.Short, &t.Status, &t.Domain, &t.Plan,
			&t.Branding.Brand.Primary, &t.Branding.Brand.Secondary, &t.Branding.LogoURL,
		)
	})
	if err != nil {
		return domain.Tenant{}, tenantNotFound(err, code)
	}
	return t, nil
}

// CreateTenant inserts a new tenant and seeds default feature flags from the
// canonical catalog with all switches disabled.
func (r *Repository) CreateTenant(ctx context.Context, t domain.Tenant) error {
	return r.db.WithTx(withTenant(ctx, t.Code), func(ctx context.Context, tx pgx.Tx) error {
		if err := createTenant(ctx, tx, t); err != nil {
			return err
		}
		return enqueueTenantEvent(ctx, tx, t.Code, "tenant.created.v1", map[string]any{
			"tenant_code": t.Code,
			"name":        t.Name,
			"plan":        t.Plan,
		})
	})
}

func createTenant(ctx context.Context, tx pgx.Tx, t domain.Tenant) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO tenants (code, name, short, status, domain, plan, brand_primary, brand_secondary, logo_url, primary_contact_email)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, t.Code, t.Name, t.Short, t.Status, t.Domain, t.Plan,
		t.Branding.Brand.Primary, t.Branding.Brand.Secondary, t.Branding.LogoURL, nil); err != nil {
		return fmt.Errorf("insert tenant: %w", err)
	}
	for _, f := range domain.FeatureCatalog() {
		enabled := domain.PlanAllows(t.Plan, f.PlanRequired)
		if _, err := tx.Exec(ctx, `
			INSERT INTO tenant_features (tenant_code, feature_key, is_enabled)
			VALUES ($1, $2, $3)
		`, t.Code, f.Key, enabled); err != nil {
			return fmt.Errorf("seed feature %s: %w", f.Key, err)
		}
	}
	return nil
}

// UpdateTenant applies a partial update to a tenant row inside a single
// transaction scoped to that tenant's RLS policy.
func (r *Repository) UpdateTenant(ctx context.Context, code string, upd domain.TenantUpdate) (domain.Tenant, error) {
	var updated domain.Tenant
	err := r.db.WithTx(withTenant(ctx, code), func(ctx context.Context, tx pgx.Tx) error {
		var current domain.Tenant
		if err := tx.QueryRow(ctx, `
			SELECT code, name, COALESCE(short, ''), status, COALESCE(domain, ''), plan,
			       COALESCE(brand_primary, ''), COALESCE(brand_secondary, ''), COALESCE(logo_url, '')
			FROM tenants
			WHERE code = $1
		`, code).Scan(
			&current.Code, &current.Name, &current.Short, &current.Status, &current.Domain, &current.Plan,
			&current.Branding.Brand.Primary, &current.Branding.Brand.Secondary, &current.Branding.LogoURL,
		); err != nil {
			return tenantNotFound(err, code)
		}

		updated = current.ApplyUpdate(upd)
		if err := updated.Validate(); err != nil {
			return err
		}

		_, err := tx.Exec(ctx, `
			UPDATE tenants
			SET name = $2, short = $3, status = $4, domain = $5, plan = $6,
			    brand_primary = $7, brand_secondary = $8, logo_url = $9, updated_at = now()
			WHERE code = $1
		`, updated.Code, updated.Name, updated.Short, updated.Status, updated.Domain, updated.Plan,
			updated.Branding.Brand.Primary, updated.Branding.Brand.Secondary, updated.Branding.LogoURL)
		if err != nil {
			return fmt.Errorf("update tenant: %w", err)
		}
		return enqueueTenantEvent(ctx, tx, code, "tenant.updated.v1", map[string]any{
			"tenant_code": code,
			"name":        updated.Name,
			"status":      updated.Status,
			"plan":        updated.Plan,
		})
	})
	if err != nil {
		return domain.Tenant{}, err
	}
	return updated, nil
}

// DeleteTenant removes a tenant and its feature flags (cascade).
func (r *Repository) DeleteTenant(ctx context.Context, code string) error {
	return r.db.WithTx(withTenant(ctx, code), func(ctx context.Context, tx pgx.Tx) error {
		ct, err := tx.Exec(ctx, `DELETE FROM tenants WHERE code = $1`, code)
		if err != nil {
			return fmt.Errorf("delete tenant: %w", err)
		}
		if ct.RowsAffected() == 0 {
			return fmt.Errorf("%w: %s", domain.ErrNotFound, code)
		}
		return enqueueTenantEvent(ctx, tx, code, "tenant.deleted.v1", map[string]any{
			"tenant_code": code,
		})
	})
}

// ResolveTenant is a public lookup used before authentication. The repository
// elevates only this read-only projection inside a transaction because the
// tenant is not known until the lookup succeeds.
func (r *Repository) ResolveTenant(ctx context.Context, domainHost, subdomain string) (domain.Tenant, error) {
	var t domain.Tenant
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return domain.Tenant{}, fmt.Errorf("resolve tenant: begin: %w", err)
	}
	defer tx.Rollback(ctx)
	lookupCtx := auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	if err := db.SetPlatformAdmin(lookupCtx, tx); err != nil {
		return domain.Tenant{}, err
	}
	err = tx.QueryRow(ctx, `
		SELECT code, name, COALESCE(short, ''), status, COALESCE(domain, ''), plan, COALESCE(brand_primary, ''), COALESCE(brand_secondary, ''), COALESCE(logo_url, '')
		FROM tenants
		WHERE status = 'active'
		  AND (($1 <> '' AND lower(domain) = lower($1))
		   OR ($2 <> '' AND code = lower($2)))
		LIMIT 1
	`, domainHost, subdomain).Scan(
		&t.Code, &t.Name, &t.Short, &t.Status, &t.Domain, &t.Plan,
		&t.Branding.Brand.Primary, &t.Branding.Brand.Secondary, &t.Branding.LogoURL,
	)
	if err != nil {
		return domain.Tenant{}, tenantNotFound(err, domainHost+subdomain)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Tenant{}, fmt.Errorf("resolve tenant: commit: %w", err)
	}
	return t, nil
}

func (r *Repository) RequestCustomDomain(ctx context.Context, registration domain.CustomDomain, challengeHash string) (domain.CustomDomain, error) {
	err := r.db.WithTx(withTenant(ctx, registration.TenantCode), func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			INSERT INTO tenant_custom_domains (tenant_code, hostname, status, txt_record_name, challenge_hash)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (tenant_code) DO UPDATE SET
				hostname = EXCLUDED.hostname,
				status = EXCLUDED.status,
				txt_record_name = EXCLUDED.txt_record_name,
				challenge_hash = EXCLUDED.challenge_hash,
				verified_at = NULL,
				activated_at = NULL,
				deactivated_at = NULL,
				provider_reference = NULL,
				updated_at = now()
			WHERE tenant_custom_domains.status <> 'active'
		`, registration.TenantCode, registration.Hostname, registration.Status, registration.TXTRecordName, challengeHash)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return domain.ErrConflict
			}
			return fmt.Errorf("request custom domain: %w", err)
		}
		if result.RowsAffected() == 0 {
			return domain.ErrConflict
		}
		return nil
	})
	if err != nil {
		return domain.CustomDomain{}, err
	}
	return registration, nil
}

func (r *Repository) GetCustomDomain(ctx context.Context, code string) (domain.CustomDomain, string, error) {
	var registration domain.CustomDomain
	var challengeHash string
	err := r.db.WithTx(withTenant(ctx, code), func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT tenant_code, hostname, status, txt_record_name, challenge_hash,
			       verified_at, activated_at, deactivated_at, COALESCE(provider_reference, '')
			FROM tenant_custom_domains WHERE tenant_code = $1
		`, code).Scan(&registration.TenantCode, &registration.Hostname, &registration.Status,
			&registration.TXTRecordName, &challengeHash, &registration.VerifiedAt,
			&registration.ActivatedAt, &registration.DeactivatedAt, &registration.ProviderReference)
	})
	if err != nil {
		return domain.CustomDomain{}, "", tenantNotFound(err, code)
	}
	return registration, challengeHash, nil
}

func (r *Repository) MarkCustomDomainVerified(ctx context.Context, code string, verifiedAt time.Time) (domain.CustomDomain, error) {
	err := r.db.WithTx(withTenant(ctx, code), func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE tenant_custom_domains
			SET status = 'verified', verified_at = $2, updated_at = now()
			WHERE tenant_code = $1 AND status IN ('pending_dns', 'verified')
		`, code, verifiedAt)
		if err != nil {
			return fmt.Errorf("verify custom domain: %w", err)
		}
		if result.RowsAffected() == 0 {
			return domain.ErrConflict
		}
		return nil
	})
	if err != nil {
		return domain.CustomDomain{}, err
	}
	registration, _, err := r.GetCustomDomain(ctx, code)
	return registration, err
}

func (r *Repository) ActivateCustomDomain(ctx context.Context, code, providerReference string, activatedAt time.Time) (domain.CustomDomain, error) {
	err := r.db.WithTx(withTenant(ctx, code), func(ctx context.Context, tx pgx.Tx) error {
		var hostname string
		if err := tx.QueryRow(ctx, `
			UPDATE tenant_custom_domains
			SET status = 'active', activated_at = $2, provider_reference = $3, updated_at = now()
			WHERE tenant_code = $1 AND status = 'verified'
			RETURNING hostname
		`, code, activatedAt, providerReference).Scan(&hostname); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrConflict
			}
			return fmt.Errorf("activate custom domain: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE tenants SET domain = $2, updated_at = now() WHERE code = $1`, code, hostname); err != nil {
			return fmt.Errorf("activate custom domain tenant: %w", err)
		}
		return enqueueTenantEvent(ctx, tx, code, "tenant.custom_domain_activated.v1", map[string]any{
			"tenant_code": code, "hostname": hostname,
		})
	})
	if err != nil {
		return domain.CustomDomain{}, err
	}
	registration, _, err := r.GetCustomDomain(ctx, code)
	return registration, err
}

func (r *Repository) DeactivateCustomDomain(ctx context.Context, code, providerReference string, deactivatedAt time.Time) (domain.CustomDomain, error) {
	err := r.db.WithTx(withTenant(ctx, code), func(ctx context.Context, tx pgx.Tx) error {
		var hostname string
		if err := tx.QueryRow(ctx, `
			UPDATE tenant_custom_domains
			SET status = 'inactive', deactivated_at = $2, provider_reference = $3, updated_at = now()
			WHERE tenant_code = $1 AND status = 'active'
			RETURNING hostname
		`, code, deactivatedAt, providerReference).Scan(&hostname); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrConflict
			}
			return fmt.Errorf("deactivate custom domain: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE tenants SET domain = NULL, updated_at = now() WHERE code = $1 AND domain = $2`, code, hostname); err != nil {
			return fmt.Errorf("deactivate custom domain tenant: %w", err)
		}
		return enqueueTenantEvent(ctx, tx, code, "tenant.custom_domain_deactivated.v1", map[string]any{
			"tenant_code": code, "hostname": hostname,
		})
	})
	if err != nil {
		return domain.CustomDomain{}, err
	}
	registration, _, err := r.GetCustomDomain(ctx, code)
	return registration, err
}

// Features returns the feature snapshot for a tenant, joining persisted state
// with the canonical catalog so plan_required is always present.
func (r *Repository) Features(ctx context.Context, code string) ([]domain.FeatureFlag, error) {
	catalog := domain.FeatureCatalog()
	out := make([]domain.FeatureFlag, 0, len(catalog))
	for _, f := range catalog {
		out = append(out, domain.FeatureFlag{Key: f.Key, PlanRequired: f.PlanRequired})
	}

	err := r.db.WithTx(withTenant(ctx, code), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT feature_key, is_enabled, config,
			       rollout_percentage, rollout_updated_by, rollout_reason
			FROM tenant_features
			WHERE tenant_code = $1
		`, code)
		if err != nil {
			return fmt.Errorf("query features: %w", err)
		}
		defer rows.Close()

		found := false
		for rows.Next() {
			found = true
			var key string
			var enabled bool
			var configBytes []byte
			var pct *int
			var updatedBy, rolloutReason *string
			if err := rows.Scan(&key, &enabled, &configBytes, &pct, &updatedBy, &rolloutReason); err != nil {
				return fmt.Errorf("scan feature: %w", err)
			}
			if err := applyFeatureRow(out, key, enabled, configBytes, pct, updatedBy, rolloutReason); err != nil {
				return err
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate features: %w", err)
		}
		if !found {
			// The tenant_features rows only exist after a tenant is created. If no
			// rows are returned the tenant either does not exist or is invisible to
			// this tenant scope.
			return domain.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SetFeature upserts a tenant feature flag and returns the updated flag.
func (r *Repository) SetFeature(ctx context.Context, code, key string, enabled bool, reason string) (domain.FeatureFlag, error) {
	plan, known := domain.FeaturePlan(key)
	if !known {
		return domain.FeatureFlag{}, domain.ErrValidation
	}

	err := r.db.WithTx(withTenant(ctx, code), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO tenant_features (tenant_code, feature_key, is_enabled, reason)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (tenant_code, feature_key)
			DO UPDATE SET is_enabled = EXCLUDED.is_enabled, reason = EXCLUDED.reason, updated_at = now()
		`, code, key, enabled, reason)
		if err != nil {
			return fmt.Errorf("upsert feature: %w", err)
		}
		eventType := "tenant.feature_disabled.v1"
		if enabled {
			eventType = "tenant.feature_enabled.v1"
		}
		return enqueueTenantEvent(ctx, tx, code, eventType, map[string]any{
			"feature_key": key,
			"is_enabled":  enabled,
			"plan":        plan,
		})
	})
	if err != nil {
		return domain.FeatureFlag{}, err
	}
	return domain.FeatureFlag{Key: key, Enabled: enabled, PlanRequired: plan}, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Settings returns a tenant's operational settings, falling back to empty
// defaults when the columns have not been seeded yet.
func (r *Repository) Settings(ctx context.Context, code string) (domain.Settings, error) {
	var s domain.Settings
	err := r.db.WithTx(withTenant(ctx, code), func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COALESCE(locale, ''),
			       COALESCE(timezone, ''),
			       COALESCE(date_format, ''),
			       COALESCE(academic_year_start_month, 0),
			       COALESCE(primary_contact_email, '')
			FROM tenants
			WHERE code = $1
		`, code).Scan(&s.Locale, &s.Timezone, &s.DateFormat, &s.AcademicYearStartMonth, &s.PrimaryContactEmail)
	})
	if err != nil {
		return domain.Settings{}, tenantNotFound(err, code)
	}
	return s, nil
}

// UpdateSettings applies a tenant's operational settings.
func (r *Repository) UpdateSettings(ctx context.Context, code string, s domain.Settings) error {
	return r.db.WithTx(withTenant(ctx, code), func(ctx context.Context, tx pgx.Tx) error {
		ct, err := tx.Exec(ctx, `
			UPDATE tenants
			SET locale = $2, timezone = $3, date_format = $4,
			    academic_year_start_month = $5, primary_contact_email = $6, updated_at = now()
			WHERE code = $1
		`, code, s.Locale, s.Timezone, s.DateFormat, s.AcademicYearStartMonth, s.PrimaryContactEmail)
		if err != nil {
			return fmt.Errorf("update settings: %w", err)
		}
		if ct.RowsAffected() == 0 {
			return fmt.Errorf("%w: %s", domain.ErrNotFound, code)
		}
		return enqueueTenantEvent(ctx, tx, code, "tenant.settings_updated.v1", map[string]any{
			"tenant_code": code,
			"locale":      s.Locale,
			"timezone":    s.Timezone,
		})
	})
}

func applyFeatureRow(out []domain.FeatureFlag, key string, enabled bool, configBytes []byte, pct *int, updatedBy, rolloutReason *string) error {
	for i := range out {
		if out[i].Key != key {
			continue
		}
		out[i].Enabled = enabled
		if len(configBytes) > 0 {
			if err := json.Unmarshal(configBytes, &out[i].Config); err != nil {
				return fmt.Errorf("decode feature config: %w", err)
			}
		}
		if pct != nil {
			out[i].Rollout = &domain.RolloutConfig{
				Percentage: *pct,
				UpdatedBy:  deref(updatedBy),
				Reason:     deref(rolloutReason),
			}
		}
		return nil
	}
	return nil
}
