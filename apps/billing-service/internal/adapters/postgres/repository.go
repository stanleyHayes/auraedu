// Package postgres provides the Postgres implementations of the billing repository ports.
//
//nolint:misspell // British spelling "cancelled" is intentional for the billing domain.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/billing-service/internal/domain"
	"github.com/auraedu/billing-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
)

// PlanRepository is the Postgres implementation of ports.PlanRepository.
type PlanRepository struct{ db *db.DB }

// SubscriptionRepository is the Postgres implementation of ports.SubscriptionRepository.
type SubscriptionRepository struct{ db *db.DB }

// SaaSInvoiceRepository is the Postgres implementation of ports.SaaSInvoiceRepository.
type SaaSInvoiceRepository struct{ db *db.DB }

var (
	_ ports.PlanRepository         = (*PlanRepository)(nil)
	_ ports.SubscriptionRepository = (*SubscriptionRepository)(nil)
	_ ports.SaaSInvoiceRepository  = (*SaaSInvoiceRepository)(nil)
)

// NewPlanRepository creates a Postgres-backed plan repository.
func NewPlanRepository(database *db.DB) *PlanRepository { return &PlanRepository{db: database} }

// NewSubscriptionRepository creates a Postgres-backed subscription repository.
func NewSubscriptionRepository(database *db.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: database}
}

// NewSaaSInvoiceRepository creates a Postgres-backed invoice repository.
func NewSaaSInvoiceRepository(database *db.DB) *SaaSInvoiceRepository {
	return &SaaSInvoiceRepository{db: database}
}

// --- Plan persistence ---

func (r *PlanRepository) Create(ctx context.Context, p *domain.Plan) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO billing_plans (id, name, code, description, price_cents, currency, billing_interval, features, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, p.ID, p.Name, p.Code, p.Description, p.PriceCents, p.Currency, p.BillingInterval, p.Features, p.Status, p.CreatedAt, p.UpdatedAt)
		if err != nil {
			return fmt.Errorf("billing: create plan: %w", err)
		}
		return nil
	})
}

func (r *PlanRepository) GetByID(ctx context.Context, id string) (*domain.Plan, error) {
	var p *domain.Plan
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, name, code, description, price_cents, currency, billing_interval, features, status, created_at, updated_at
			FROM billing_plans
			WHERE id = $1
		`, id)
		got, err := scanPlan(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("billing: get plan: %w", err)
		}
		p = got
		return nil
	})
	return p, err
}

func (r *PlanRepository) GetByCode(ctx context.Context, code string) (*domain.Plan, error) {
	var p *domain.Plan
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, name, code, description, price_cents, currency, billing_interval, features, status, created_at, updated_at
			FROM billing_plans
			WHERE code = LOWER(TRIM($1))
		`, code)
		got, err := scanPlan(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("billing: get plan by code: %w", err)
		}
		p = got
		return nil
	})
	return p, err
}

func (r *PlanRepository) List(ctx context.Context, filter ports.PlanFilter) ([]*domain.Plan, string, error) {
	var out []*domain.Plan
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listPlansQuery(ctx, tx, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			p, err := scanPlan(rows)
			if err != nil {
				return err
			}
			out = append(out, p)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("billing: list plans rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listPlansQuery(ctx context.Context, tx pgx.Tx, filter ports.PlanFilter) (pgx.Rows, error) {
	args := []any{}
	where := "1 = 1"
	idx := 0

	if filter.Status != "" {
		idx++
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", idx)
	}
	if filter.Cursor != "" {
		idx++
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM billing_plans WHERE id = $%d)", idx)
	}

	idx++
	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, name, code, description, price_cents, currency, billing_interval, features, status, created_at, updated_at
		FROM billing_plans
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, idx)
	return tx.Query(ctx, sql, args...)
}

func (r *PlanRepository) Update(ctx context.Context, p *domain.Plan) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE billing_plans
			SET name = $2, code = $3, description = $4, price_cents = $5, currency = $6,
			    billing_interval = $7, features = $8, status = $9, updated_at = $10
			WHERE id = $1
		`, p.ID, p.Name, p.Code, p.Description, p.PriceCents, p.Currency, p.BillingInterval, p.Features, p.Status, p.UpdatedAt)
		if err != nil {
			return fmt.Errorf("billing: update plan: %w", err)
		}
		return nil
	})
}

func (r *PlanRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM billing_plans WHERE id = $1`, id)
		if err != nil {
			return fmt.Errorf("billing: delete plan: %w", err)
		}
		return nil
	})
}

// --- Subscription persistence ---

func (r *SubscriptionRepository) Create(ctx context.Context, tenantID string, s *domain.Subscription) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO billing_subscriptions (
				id, tenant_id, plan_id, status, current_period_start, current_period_end,
				trial_ends_at, cancelled_at, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, s.ID, tenantID, s.PlanID, s.Status, s.CurrentPeriodStart, s.CurrentPeriodEnd, s.TrialEndsAt, s.CancelledAt, s.CreatedAt, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("billing: create subscription: %w", err)
		}
		return nil
	})
}

func (r *SubscriptionRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	var s *domain.Subscription
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, plan_id, status, current_period_start, current_period_end, trial_ends_at, cancelled_at, created_at, updated_at
			FROM billing_subscriptions
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanSubscription(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("billing: get subscription: %w", err)
		}
		s = got
		return nil
	})
	return s, err
}

func (r *SubscriptionRepository) List(ctx context.Context, tenantID string, filter ports.SubscriptionFilter) ([]*domain.Subscription, string, error) {
	var out []*domain.Subscription
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listSubscriptionsQuery(ctx, tx, tenantID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			s, err := scanSubscription(rows)
			if err != nil {
				return err
			}
			out = append(out, s)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("billing: list subscriptions rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listSubscriptionsQuery(ctx context.Context, tx pgx.Tx, tenantID string, filter ports.SubscriptionFilter) (pgx.Rows, error) {
	args := []any{tenantID}
	where := "tenant_id = $1"

	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.PlanID != "" {
		args = append(args, filter.PlanID)
		where += fmt.Sprintf(" AND plan_id = $%d", len(args))
	}
	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM billing_subscriptions WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, plan_id, status, current_period_start, current_period_end, trial_ends_at, cancelled_at, created_at, updated_at
		FROM billing_subscriptions
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *SubscriptionRepository) Update(ctx context.Context, tenantID string, s *domain.Subscription) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE billing_subscriptions
			SET plan_id = $3, status = $4, current_period_start = $5, current_period_end = $6,
			    trial_ends_at = $7, cancelled_at = $8, updated_at = $9
			WHERE id = $1 AND tenant_id = $2
		`, s.ID, tenantID, s.PlanID, s.Status, s.CurrentPeriodStart, s.CurrentPeriodEnd, s.TrialEndsAt, s.CancelledAt, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("billing: update subscription: %w", err)
		}
		return nil
	})
}

func (r *SubscriptionRepository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM billing_subscriptions WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		if err != nil {
			return fmt.Errorf("billing: delete subscription: %w", err)
		}
		return nil
	})
}

// --- SaaSInvoice persistence ---

func (r *SaaSInvoiceRepository) Create(ctx context.Context, tenantID string, i *domain.SaaSInvoice) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO billing_invoices (id, tenant_id, subscription_id, amount_cents, status, due_date, paid_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, i.ID, tenantID, i.SubscriptionID, i.AmountCents, i.Status, datePtr(i.DueDate), i.PaidAt, i.CreatedAt, i.UpdatedAt)
		if err != nil {
			return fmt.Errorf("billing: create invoice: %w", err)
		}
		return nil
	})
}

func (r *SaaSInvoiceRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.SaaSInvoice, error) {
	var inv *domain.SaaSInvoice
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, subscription_id, amount_cents, status, due_date, paid_at, created_at, updated_at
			FROM billing_invoices
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanSaaSInvoice(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("billing: get invoice: %w", err)
		}
		inv = got
		return nil
	})
	return inv, err
}

func (r *SaaSInvoiceRepository) List(ctx context.Context, tenantID string, filter ports.SaaSInvoiceFilter) ([]*domain.SaaSInvoice, string, error) {
	var out []*domain.SaaSInvoice
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listSaaSInvoicesQuery(ctx, tx, tenantID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			inv, err := scanSaaSInvoice(rows)
			if err != nil {
				return err
			}
			out = append(out, inv)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("billing: list invoices rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listSaaSInvoicesQuery(ctx context.Context, tx pgx.Tx, tenantID string, filter ports.SaaSInvoiceFilter) (pgx.Rows, error) {
	args := []any{tenantID}
	where := "tenant_id = $1"

	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.SubscriptionID != "" {
		args = append(args, filter.SubscriptionID)
		where += fmt.Sprintf(" AND subscription_id = $%d", len(args))
	}
	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM billing_invoices WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, subscription_id, amount_cents, status, due_date, paid_at, created_at, updated_at
		FROM billing_invoices
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *SaaSInvoiceRepository) Update(ctx context.Context, tenantID string, i *domain.SaaSInvoice) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE billing_invoices
			SET subscription_id = $3, amount_cents = $4, status = $5, due_date = $6, paid_at = $7, updated_at = $8
			WHERE id = $1 AND tenant_id = $2
		`, i.ID, tenantID, i.SubscriptionID, i.AmountCents, i.Status, datePtr(i.DueDate), i.PaidAt, i.UpdatedAt)
		if err != nil {
			return fmt.Errorf("billing: update invoice: %w", err)
		}
		return nil
	})
}

func (r *SaaSInvoiceRepository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM billing_invoices WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		if err != nil {
			return fmt.Errorf("billing: delete invoice: %w", err)
		}
		return nil
	})
}

// --- helpers ---

type scanner interface {
	Scan(dest ...any) error
}

func scanPlan(row scanner) (*domain.Plan, error) {
	var p domain.Plan
	if err := row.Scan(
		&p.ID, &p.Name, &p.Code, &p.Description, &p.PriceCents, &p.Currency,
		&p.BillingInterval, &p.Features, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}

func scanSubscription(row scanner) (*domain.Subscription, error) {
	var s domain.Subscription
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.PlanID, &s.Status, &s.CurrentPeriodStart, &s.CurrentPeriodEnd,
		&s.TrialEndsAt, &s.CancelledAt, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}

func scanSaaSInvoice(row scanner) (*domain.SaaSInvoice, error) {
	var i domain.SaaSInvoice
	var dueDate *time.Time
	if err := row.Scan(
		&i.ID, &i.TenantID, &i.SubscriptionID, &i.AmountCents, &i.Status,
		&dueDate, &i.PaidAt, &i.CreatedAt, &i.UpdatedAt,
	); err != nil {
		return nil, err
	}
	i.DueDate = dueDate
	return &i, nil
}

func datePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}
