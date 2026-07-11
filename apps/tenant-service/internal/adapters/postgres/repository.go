// Package postgres is a PostgreSQL+RLS implementation of ports.Repository.
// It uses platform/db so app.tenant_id is set for every transaction, enforcing
// row-level isolation keyed on tenant code.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/auraedu/tenant-service/internal/ports"
	"github.com/jackc/pgx/v5"
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
	rows, err := r.db.Pool().Query(ctx, `
		SELECT code, name, short, status, domain, plan, brand_primary, brand_secondary, logo_url
		FROM tenants
		ORDER BY created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var out []domain.Tenant
	for rows.Next() {
		var t domain.Tenant
		if err := rows.Scan(
			&t.Code, &t.Name, &t.Short, &t.Status, &t.Domain, &t.Plan,
			&t.Branding.Brand.Primary, &t.Branding.Brand.Secondary, &t.Branding.LogoURL,
		); err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenants: %w", err)
	}
	return out, nil
}

// GetTenant returns a tenant by code.
func (r *Repository) GetTenant(ctx context.Context, code string) (domain.Tenant, error) {
	var t domain.Tenant
	err := r.db.WithTx(withTenant(ctx, code), func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT code, name, short, status, domain, plan, brand_primary, brand_secondary, logo_url
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
		if _, err := tx.Exec(ctx, `
			INSERT INTO tenants (code, name, short, status, domain, plan, brand_primary, brand_secondary, logo_url)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, t.Code, t.Name, t.Short, t.Status, t.Domain, t.Plan,
			t.Branding.Brand.Primary, t.Branding.Brand.Secondary, t.Branding.LogoURL); err != nil {
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
	})
}

// UpdateTenant applies a partial update to a tenant row inside a single
// transaction scoped to that tenant's RLS policy.
func (r *Repository) UpdateTenant(ctx context.Context, code string, upd domain.TenantUpdate) (domain.Tenant, error) {
	var updated domain.Tenant
	err := r.db.WithTx(withTenant(ctx, code), func(ctx context.Context, tx pgx.Tx) error {
		var current domain.Tenant
		if err := tx.QueryRow(ctx, `
			SELECT code, name, short, status, domain, plan, brand_primary, brand_secondary, logo_url
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
		return nil
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
		return nil
	})
}

// ResolveTenant is a public lookup used before authentication. It searches across
// all tenant rows, so it uses the raw pool rather than tenant-scoped transactions.
func (r *Repository) ResolveTenant(ctx context.Context, domainHost, subdomain string) (domain.Tenant, error) {
	var t domain.Tenant
	err := r.db.Pool().QueryRow(ctx, `
		SELECT code, name, short, status, domain, plan, brand_primary, brand_secondary, logo_url
		FROM tenants
		WHERE ($1 <> '' AND lower(domain) = lower($1))
		   OR ($2 <> '' AND code = lower($2))
		LIMIT 1
	`, domainHost, subdomain).Scan(
		&t.Code, &t.Name, &t.Short, &t.Status, &t.Domain, &t.Plan,
		&t.Branding.Brand.Primary, &t.Branding.Brand.Secondary, &t.Branding.LogoURL,
	)
	if err != nil {
		return domain.Tenant{}, tenantNotFound(err, domainHost+subdomain)
	}
	return t, nil
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
		return nil
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
