package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/payment-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
)

// PaymentRepository is the Postgres implementation of ports.PaymentRepository.
type PaymentRepository struct {
	db *db.DB
}

// TransactionRepository is the Postgres implementation of ports.TransactionRepository.
type TransactionRepository struct {
	db *db.DB
}

// WebhookEventRepository is the Postgres implementation of ports.WebhookEventRepository.
type WebhookEventRepository struct {
	db *db.DB
}

var (
	_ ports.PaymentRepository      = (*PaymentRepository)(nil)
	_ ports.TransactionRepository  = (*TransactionRepository)(nil)
	_ ports.WebhookEventRepository = (*WebhookEventRepository)(nil)
)

// NewPaymentRepository creates a Postgres-backed payment repository.
func NewPaymentRepository(database *db.DB) *PaymentRepository {
	return &PaymentRepository{db: database}
}

// NewTransactionRepository creates a Postgres-backed transaction repository.
func NewTransactionRepository(database *db.DB) *TransactionRepository {
	return &TransactionRepository{db: database}
}

// NewWebhookEventRepository creates a Postgres-backed webhook event repository.
func NewWebhookEventRepository(database *db.DB) *WebhookEventRepository {
	return &WebhookEventRepository{db: database}
}

// --- Payment persistence ---

func (r *PaymentRepository) Create(ctx context.Context, tenantID string, p *domain.Payment) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO payments (id, tenant_id, invoice_id, amount_cents, currency, provider, provider_reference, status, metadata, initiated_at, completed_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9, '{}'::jsonb), $10, $11, $12, $13)
		`, p.ID, tenantID, p.InvoiceID, p.AmountCents, p.Currency, p.Provider, p.ProviderReference, p.Status, p.Metadata, p.InitiatedAt, p.CompletedAt, p.CreatedAt, p.UpdatedAt)
		if err != nil {
			return fmt.Errorf("payments: create payment: %w", err)
		}
		return nil
	})
}

func (r *PaymentRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Payment, error) {
	var p *domain.Payment
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, invoice_id, amount_cents, currency, provider, provider_reference, status, metadata, initiated_at, completed_at, created_at, updated_at
			FROM payments
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanPayment(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("payments: get payment: %w", err)
		}
		p = got
		return nil
	})
	return p, err
}

func (r *PaymentRepository) GetByProviderReference(ctx context.Context, tenantID, provider, reference string) (*domain.Payment, error) {
	var p *domain.Payment
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, invoice_id, amount_cents, currency, provider, provider_reference, status, metadata, initiated_at, completed_at, created_at, updated_at
			FROM payments
			WHERE tenant_id = $1 AND provider = $2 AND provider_reference = $3
		`, tenantID, provider, reference)
		got, err := scanPayment(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("payments: get payment by provider reference: %w", err)
		}
		p = got
		return nil
	})
	return p, err
}

func (r *PaymentRepository) List(ctx context.Context, tenantID string, filter ports.PaymentFilter) ([]*domain.Payment, string, error) {
	var out []*domain.Payment
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listPaymentsQuery(ctx, tx, tenantID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			p, err := scanPayment(rows)
			if err != nil {
				return err
			}
			out = append(out, p)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("payments: list payments rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listPaymentsQuery(ctx context.Context, tx pgx.Tx, tenantID string, filter ports.PaymentFilter) (pgx.Rows, error) {
	args := []any{tenantID}
	where := "tenant_id = $1"

	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.Provider != "" {
		args = append(args, filter.Provider)
		where += fmt.Sprintf(" AND provider = $%d", len(args))
	}
	if filter.InvoiceID != "" {
		args = append(args, filter.InvoiceID)
		where += fmt.Sprintf(" AND invoice_id = $%d", len(args))
	}
	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM payments WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, invoice_id, amount_cents, currency, provider, provider_reference, status, metadata, initiated_at, completed_at, created_at, updated_at
		FROM payments
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *PaymentRepository) Update(ctx context.Context, tenantID string, p *domain.Payment) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE payments
			SET invoice_id = $3, amount_cents = $4, currency = $5, provider = $6, provider_reference = $7, status = $8, metadata = COALESCE($9, '{}'::jsonb), initiated_at = $10, completed_at = $11, updated_at = $12
			WHERE id = $1 AND tenant_id = $2
		`, p.ID, tenantID, p.InvoiceID, p.AmountCents, p.Currency, p.Provider, p.ProviderReference, p.Status, p.Metadata, p.InitiatedAt, p.CompletedAt, p.UpdatedAt)
		if err != nil {
			return fmt.Errorf("payments: update payment: %w", err)
		}
		return nil
	})
}

func (r *PaymentRepository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			DELETE FROM payments
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		if err != nil {
			return fmt.Errorf("payments: delete payment: %w", err)
		}
		return nil
	})
}

// --- Transaction persistence ---

func (r *TransactionRepository) Create(ctx context.Context, tenantID string, t *domain.Transaction) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO transactions (id, tenant_id, payment_id, type, status, amount_cents, reference, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, t.ID, tenantID, t.PaymentID, t.Type, t.Status, t.AmountCents, t.Reference, t.CreatedAt)
		if err != nil {
			return fmt.Errorf("payments: create transaction: %w", err)
		}
		return nil
	})
}

func (r *TransactionRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Transaction, error) {
	var t *domain.Transaction
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, payment_id, type, status, amount_cents, reference, created_at
			FROM transactions
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanTransaction(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("payments: get transaction: %w", err)
		}
		t = got
		return nil
	})
	return t, err
}

func (r *TransactionRepository) ListByPayment(ctx context.Context, tenantID, paymentID string, filter ports.TransactionFilter) ([]*domain.Transaction, string, error) {
	var out []*domain.Transaction
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		args := []any{tenantID, paymentID}
		where := "tenant_id = $1 AND payment_id = $2"
		if filter.Cursor != "" {
			args = append(args, filter.Cursor)
			where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM transactions WHERE id = $%d AND tenant_id = $1)", len(args))
		}
		args = append(args, filter.Limit)
		sql := fmt.Sprintf(`
			SELECT id, tenant_id, payment_id, type, status, amount_cents, reference, created_at
			FROM transactions
			WHERE %s
			ORDER BY created_at ASC, id ASC
			LIMIT $%d
		`, where, len(args))

		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			t, err := scanTransaction(rows)
			if err != nil {
				return err
			}
			out = append(out, t)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("payments: list transactions rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

// --- WebhookEvent persistence ---

func (r *WebhookEventRepository) Create(ctx context.Context, tenantID string, w *domain.WebhookEvent) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO webhook_events (id, tenant_id, provider, event_type, payload, signature, processed, processed_at, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, w.ID, tenantID, w.Provider, w.EventType, w.Payload, w.Signature, w.Processed, w.ProcessedAt, w.CreatedAt)
		if err != nil {
			return fmt.Errorf("payments: create webhook event: %w", err)
		}
		return nil
	})
}

func (r *WebhookEventRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.WebhookEvent, error) {
	var w *domain.WebhookEvent
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, provider, event_type, payload, signature, processed, processed_at, created_at
			FROM webhook_events
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanWebhookEvent(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("payments: get webhook event: %w", err)
		}
		w = got
		return nil
	})
	return w, err
}

func (r *WebhookEventRepository) Update(ctx context.Context, tenantID string, w *domain.WebhookEvent) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE webhook_events
			SET provider = $3, event_type = $4, payload = $5, signature = $6, processed = $7, processed_at = $8
			WHERE id = $1 AND tenant_id = $2
		`, w.ID, tenantID, w.Provider, w.EventType, w.Payload, w.Signature, w.Processed, w.ProcessedAt)
		if err != nil {
			return fmt.Errorf("payments: update webhook event: %w", err)
		}
		return nil
	})
}

func (r *WebhookEventRepository) List(ctx context.Context, tenantID string, filter ports.WebhookEventFilter) ([]*domain.WebhookEvent, string, error) {
	var out []*domain.WebhookEvent
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		args := []any{tenantID}
		where := "tenant_id = $1"
		if filter.Provider != "" {
			args = append(args, filter.Provider)
			where += fmt.Sprintf(" AND provider = $%d", len(args))
		}
		if filter.EventType != "" {
			args = append(args, filter.EventType)
			where += fmt.Sprintf(" AND event_type = $%d", len(args))
		}
		if filter.Cursor != "" {
			args = append(args, filter.Cursor)
			where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM webhook_events WHERE id = $%d AND tenant_id = $1)", len(args))
		}
		args = append(args, filter.Limit)
		sql := fmt.Sprintf(`
			SELECT id, tenant_id, provider, event_type, payload, signature, processed, processed_at, created_at
			FROM webhook_events
			WHERE %s
			ORDER BY created_at ASC, id ASC
			LIMIT $%d
		`, where, len(args))

		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			w, err := scanWebhookEvent(rows)
			if err != nil {
				return err
			}
			out = append(out, w)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("payments: list webhook events rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanPayment(row scanner) (*domain.Payment, error) {
	var p domain.Payment
	if err := row.Scan(
		&p.ID, &p.TenantID, &p.InvoiceID, &p.AmountCents, &p.Currency, &p.Provider, &p.ProviderReference, &p.Status,
		&p.Metadata, &p.InitiatedAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}

func scanTransaction(row scanner) (*domain.Transaction, error) {
	var t domain.Transaction
	if err := row.Scan(
		&t.ID, &t.TenantID, &t.PaymentID, &t.Type, &t.Status, &t.AmountCents, &t.Reference, &t.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &t, nil
}

func scanWebhookEvent(row scanner) (*domain.WebhookEvent, error) {
	var w domain.WebhookEvent
	if err := row.Scan(
		&w.ID, &w.TenantID, &w.Provider, &w.EventType, &w.Payload, &w.Signature, &w.Processed, &w.ProcessedAt, &w.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &w, nil
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
