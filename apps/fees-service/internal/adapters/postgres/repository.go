// Package postgres provides Postgres-backed repositories for the fees service.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// FeeStructureRepository is the Postgres implementation of ports.FeeStructureRepository.
// It uses platform/db.WithTx so that app.tenant_id is set on the same connection
// that executes the query, which makes the Row-Level Security policy effective.
// Every query also filters by tenant_id explicitly as defense-in-depth.
type FeeStructureRepository struct {
	db *db.DB
}

// InvoiceRepository is the Postgres implementation of ports.InvoiceRepository.
type InvoiceRepository struct {
	db *db.DB
}

var (
	_ ports.FeeStructureRepository          = (*FeeStructureRepository)(nil)
	_ ports.InvoiceRepository               = (*InvoiceRepository)(nil)
	_ ports.BalanceRepository               = (*InvoiceRepository)(nil)
	_ ports.ReceiptRepository               = (*InvoiceRepository)(nil)
	_ ports.PaymentReconciliationRepository = (*InvoiceRepository)(nil)
	_ ports.DurablePaymentReconciliation    = (*InvoiceRepository)(nil)
	_ ports.InvoiceLifecycleRepository      = (*InvoiceRepository)(nil)
	_ ports.OutboxRepository                = (*InvoiceRepository)(nil)
)

// NewFeeStructureRepository creates a Postgres-backed fee-structure repository.
func NewFeeStructureRepository(database *db.DB) *FeeStructureRepository {
	return &FeeStructureRepository{db: database}
}

// NewInvoiceRepository creates a Postgres-backed invoice repository.
func NewInvoiceRepository(database *db.DB) *InvoiceRepository {
	return &InvoiceRepository{db: database}
}

// --- FeeStructure persistence ---

func (r *FeeStructureRepository) Create(ctx context.Context, tenantID string, fs *domain.FeeStructure) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO fee_structures (
				id, tenant_id, name, academic_year_id, amount_cents, currency,
				recurrence, target, due_day, description, status, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, fs.ID, tenantID, fs.Name, fs.AcademicYearID, fs.AmountCents, fs.Currency,
			fs.Recurrence, fs.Target, fs.DueDay, fs.Description, fs.Status, fs.CreatedAt, fs.UpdatedAt)
		if err != nil {
			return fmt.Errorf("fees: create fee structure: %w", err)
		}
		return nil
	})
}

func (r *FeeStructureRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.FeeStructure, error) {
	var fs *domain.FeeStructure
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, name, academic_year_id, amount_cents, currency, recurrence, target, due_day, description, status, created_at, updated_at
			FROM fee_structures
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanFeeStructure(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("fees: get fee structure: %w", err)
		}
		fs = got
		return nil
	})
	return fs, err
}

func (r *FeeStructureRepository) List(ctx context.Context, tenantID string, filter ports.FeeStructureFilter) ([]*domain.FeeStructure, string, error) {
	var out []*domain.FeeStructure
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listFeeStructuresQuery(ctx, tx, tenantID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			fs, err := scanFeeStructure(rows)
			if err != nil {
				return err
			}
			out = append(out, fs)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("fees: list fee structures rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listFeeStructuresQuery(ctx context.Context, tx pgx.Tx, tenantID string, filter ports.FeeStructureFilter) (pgx.Rows, error) {
	args := []any{tenantID}
	where := "tenant_id = $1"

	if filter.AcademicYearID != "" {
		args = append(args, filter.AcademicYearID)
		where += fmt.Sprintf(" AND academic_year_id = $%d", len(args))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM fee_structures WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, name, academic_year_id, amount_cents, currency, recurrence, target, due_day, description, status, created_at, updated_at
		FROM fee_structures
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *FeeStructureRepository) Update(ctx context.Context, tenantID string, fs *domain.FeeStructure) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE fee_structures
			SET name = $3, academic_year_id = $4, amount_cents = $5, currency = $6,
			    recurrence = $7, target = $8, due_day = $9, description = $10, status = $11, updated_at = $12
			WHERE id = $1 AND tenant_id = $2
		`, fs.ID, tenantID, fs.Name, fs.AcademicYearID, fs.AmountCents, fs.Currency,
			fs.Recurrence, fs.Target, fs.DueDay, fs.Description, fs.Status, fs.UpdatedAt)
		if err != nil {
			return fmt.Errorf("fees: update fee structure: %w", err)
		}
		return nil
	})
}

func (r *FeeStructureRepository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			DELETE FROM fee_structures
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		if err != nil {
			return fmt.Errorf("fees: delete fee structure: %w", err)
		}
		return nil
	})
}

// --- Invoice persistence ---

func (r *InvoiceRepository) Create(ctx context.Context, tenantID string, inv *domain.Invoice) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return insertInvoice(ctx, tx, tenantID, inv)
	})
}

func insertInvoice(ctx context.Context, tx pgx.Tx, tenantID string, inv *domain.Invoice) error {
	_, err := tx.Exec(ctx, `
			INSERT INTO invoices (
				id, tenant_id, student_id, fee_structure_id, amount_cents, balance_cents,
				status, due_date, issued_at, notes, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, inv.ID, tenantID, inv.StudentID, inv.FeeStructureID, inv.AmountCents, inv.BalanceCents,
		inv.Status, datePtr(inv.DueDate), inv.IssuedAt, inv.Notes, inv.CreatedAt, inv.UpdatedAt)
	if err != nil {
		return fmt.Errorf("fees: create invoice: %w", err)
	}
	return nil
}

func (r *InvoiceRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Invoice, error) {
	var inv *domain.Invoice
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, student_id, fee_structure_id, amount_cents, balance_cents, status, due_date, issued_at, notes, created_at, updated_at
			FROM invoices
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanInvoice(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("fees: get invoice: %w", err)
		}
		inv = got
		return nil
	})
	return inv, err
}

func (r *InvoiceRepository) List(ctx context.Context, tenantID string, filter ports.InvoiceFilter) ([]*domain.Invoice, string, error) {
	var out []*domain.Invoice
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listInvoicesQuery(ctx, tx, tenantID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			inv, err := scanInvoice(rows)
			if err != nil {
				return err
			}
			out = append(out, inv)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("fees: list invoices rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listInvoicesQuery(ctx context.Context, tx pgx.Tx, tenantID string, filter ports.InvoiceFilter) (pgx.Rows, error) {
	args := []any{tenantID}
	where := "tenant_id = $1"

	if filter.StudentID != "" {
		args = append(args, filter.StudentID)
		where += fmt.Sprintf(" AND student_id = $%d", len(args))
	}
	if filter.StudentIDs != nil {
		if len(filter.StudentIDs) == 0 {
			where += " AND false"
		} else {
			args = append(args, filter.StudentIDs)
			where += fmt.Sprintf(" AND student_id = ANY($%d)", len(args))
		}
	}
	if filter.InvoiceIDs != nil {
		if len(filter.InvoiceIDs) == 0 {
			where += " AND false"
		} else {
			args = append(args, filter.InvoiceIDs)
			where += fmt.Sprintf(" AND id = ANY($%d)", len(args))
		}
	}
	if filter.FeeStructureID != "" {
		args = append(args, filter.FeeStructureID)
		where += fmt.Sprintf(" AND fee_structure_id = $%d", len(args))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM invoices WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, student_id, fee_structure_id, amount_cents, balance_cents, status, due_date, issued_at, notes, created_at, updated_at
		FROM invoices
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *InvoiceRepository) Update(ctx context.Context, tenantID string, inv *domain.Invoice) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return updateInvoice(ctx, tx, tenantID, inv)
	})
}

func updateInvoice(ctx context.Context, tx pgx.Tx, tenantID string, inv *domain.Invoice) error {
	tag, err := tx.Exec(ctx, `
			UPDATE invoices
			SET student_id = $3, fee_structure_id = $4, amount_cents = $5, balance_cents = $6,
			    status = $7, due_date = $8, issued_at = $9, notes = $10, updated_at = $11
			WHERE id = $1 AND tenant_id = $2
		`, inv.ID, tenantID, inv.StudentID, inv.FeeStructureID, inv.AmountCents, inv.BalanceCents,
		inv.Status, datePtr(inv.DueDate), inv.IssuedAt, inv.Notes, inv.UpdatedAt)
	if err != nil {
		return fmt.Errorf("fees: update invoice: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *InvoiceRepository) CommitInvoiceLifecycle(
	ctx context.Context,
	tenantID string,
	invoice *domain.Invoice,
	mutation string,
	events []ports.LifecycleEvent,
) error {
	encoded := make([][]byte, len(events))
	for index, event := range events {
		payload, err := json.Marshal(event.Payload)
		if err != nil {
			return fmt.Errorf("fees: encode invoice lifecycle event: %w", err)
		}
		encoded[index] = payload
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		switch mutation {
		case ports.InvoiceMutationCreate:
			if err := insertInvoice(ctx, tx, tenantID, invoice); err != nil {
				return err
			}
		case ports.InvoiceMutationUpdate:
			if err := updateInvoice(ctx, tx, tenantID, invoice); err != nil {
				return err
			}
		case ports.InvoiceMutationDelete:
			tag, err := tx.Exec(ctx, `DELETE FROM invoices WHERE id=$1 AND tenant_id=$2`, invoice.ID, tenantID)
			if err != nil {
				return fmt.Errorf("fees: delete invoice: %w", err)
			}
			if tag.RowsAffected() != 1 {
				return domain.ErrNotFound
			}
		default:
			return fmt.Errorf("fees: unsupported invoice mutation %q", mutation)
		}
		for index, event := range events {
			if _, err := tx.Exec(ctx, `
				INSERT INTO fees_outbox (id,tenant_id,event_type,payload,created_at,next_attempt_at)
				VALUES ($1,$2,$3,$4,now(),now())
			`, uuid.NewString(), tenantID, event.EventType, encoded[index]); err != nil {
				return fmt.Errorf("fees: enqueue invoice lifecycle event: %w", err)
			}
		}
		return nil
	})
}

func (r *InvoiceRepository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			DELETE FROM invoices
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		if err != nil {
			return fmt.Errorf("fees: delete invoice: %w", err)
		}
		return nil
	})
}

// GetStudentBalance derives invoice totals by currency. It never combines
// unlike currencies and excludes draft/cancelled invoices from money owed.
func (r *InvoiceRepository) GetStudentBalance(ctx context.Context, tenantID, studentID string) (*domain.Balance, error) {
	balance := &domain.Balance{StudentID: studentID, Totals: []domain.CurrencyBalance{}}
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT fs.currency,
			       COALESCE(SUM(i.amount_cents), 0),
			       COALESCE(SUM(i.amount_cents - i.balance_cents), 0),
			       COALESCE(SUM(i.balance_cents), 0)
			FROM invoices i
			JOIN fee_structures fs ON fs.tenant_id = i.tenant_id AND fs.id = i.fee_structure_id
			WHERE i.tenant_id = $1 AND i.student_id = $2
			  AND i.status NOT IN ('draft', 'cancelled')
			GROUP BY fs.currency
			ORDER BY fs.currency
		`, tenantID, studentID)
		if err != nil {
			return fmt.Errorf("fees: get balance: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var total domain.CurrencyBalance
			if err := rows.Scan(&total.Currency, &total.TotalInvoicedCents, &total.TotalPaidCents, &total.OutstandingCents); err != nil {
				return fmt.Errorf("fees: scan balance: %w", err)
			}
			balance.Totals = append(balance.Totals, total)
		}
		return rows.Err()
	})
	return balance, err
}

// GetReceiptByID returns tenant-scoped immutable reconciliation evidence.
func (r *InvoiceRepository) GetReceiptByID(ctx context.Context, tenantID, id string) (*domain.Receipt, error) {
	var receipt *domain.Receipt
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		got, err := scanReceipt(tx.QueryRow(ctx, `
			SELECT id, tenant_id, invoice_id, student_id, payment_id, amount_cents,
			       applied_cents, overpayment_cents, currency, provider_reference, issued_at
			FROM receipts WHERE id = $1 AND tenant_id = $2
		`, id, tenantID))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("fees: get receipt: %w", err)
		}
		receipt = got
		return nil
	})
	return receipt, err
}

// ApplyPayment updates the invoice balance and inserts its immutable receipt in
// one tenant-scoped transaction. A transaction advisory lock makes replay safe
// even when the same payment is delivered concurrently.
func (r *InvoiceRepository) ApplyPayment(ctx context.Context, tenantID string, input ports.PaymentApplication) (*domain.Invoice, *domain.Receipt, bool, error) {
	var invoice *domain.Invoice
	var receipt *domain.Receipt
	created := false
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, tenantID+":"+input.PaymentID); err != nil {
			return fmt.Errorf("fees: lock payment reconciliation: %w", err)
		}
		existing, err := scanReceipt(tx.QueryRow(ctx, `
			SELECT id, tenant_id, invoice_id, student_id, payment_id, amount_cents,
			       applied_cents, overpayment_cents, currency, provider_reference, issued_at
			FROM receipts WHERE tenant_id = $1 AND payment_id = $2
		`, tenantID, input.PaymentID))
		if err == nil {
			receipt = existing
			invoice, err = scanInvoice(tx.QueryRow(ctx, `
				SELECT id, tenant_id, student_id, fee_structure_id, amount_cents, balance_cents,
				       status, due_date, issued_at, notes, created_at, updated_at
				FROM invoices WHERE id = $1 AND tenant_id = $2
			`, existing.InvoiceID, tenantID))
			return err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("fees: find payment receipt: %w", err)
		}

		var currency string
		var dueDate *time.Time
		var inv domain.Invoice
		err = tx.QueryRow(ctx, `
			SELECT i.id, i.tenant_id, i.student_id, i.fee_structure_id, i.amount_cents,
			       i.balance_cents, i.status, i.due_date, i.issued_at, i.notes,
			       i.created_at, i.updated_at, fs.currency
			FROM invoices i
			JOIN fee_structures fs ON fs.tenant_id = i.tenant_id AND fs.id = i.fee_structure_id
			WHERE i.id = $1 AND i.tenant_id = $2
			FOR UPDATE OF i
		`, input.InvoiceID, tenantID).Scan(
			&inv.ID, &inv.TenantID, &inv.StudentID, &inv.FeeStructureID, &inv.AmountCents,
			&inv.BalanceCents, &inv.Status, &dueDate, &inv.IssuedAt, &inv.Notes,
			&inv.CreatedAt, &inv.UpdatedAt, &currency,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("fees: lock invoice for payment: %w", err)
		}
		if dueDate != nil {
			inv.DueDate = domain.Date{Time: *dueDate}
		}

		applied := input.AmountCents
		if applied > inv.BalanceCents {
			applied = inv.BalanceCents
		}
		receipt, err = domain.NewReceipt(
			tenantID, inv.ID, inv.StudentID, input.PaymentID, currency,
			input.AmountCents, applied, input.ProviderReference, input.ReceivedAt,
		)
		if err != nil {
			return err
		}
		inv.BalanceCents -= applied
		if inv.BalanceCents == 0 {
			inv.Status = string(domain.InvoiceStatusPaid)
		} else {
			inv.Status = string(domain.InvoiceStatusPartial)
		}
		inv.UpdatedAt = time.Now().UTC()
		if _, err := tx.Exec(ctx, `
			UPDATE invoices SET balance_cents = $3, status = $4, updated_at = $5
			WHERE id = $1 AND tenant_id = $2
		`, inv.ID, tenantID, inv.BalanceCents, inv.Status, inv.UpdatedAt); err != nil {
			return fmt.Errorf("fees: apply payment to invoice: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO receipts (
				id, tenant_id, invoice_id, student_id, payment_id, amount_cents,
				applied_cents, overpayment_cents, currency, provider_reference, issued_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`, receipt.ID, tenantID, receipt.InvoiceID, receipt.StudentID, receipt.PaymentID,
			receipt.AmountCents, receipt.AppliedCents, receipt.OverpaymentCents,
			receipt.Currency, receipt.ProviderReference, receipt.IssuedAt); err != nil {
			return fmt.Errorf("fees: create receipt: %w", err)
		}
		meta := map[string]any{"payment_id": input.PaymentID, "receipt_id": receipt.ID, "applied_cents": receipt.AppliedCents}
		if err := enqueueInvoiceEvent(ctx, tx, tenantID, "invoice.updated.v1", &inv, meta); err != nil {
			return err
		}
		if inv.Status == string(domain.InvoiceStatusPaid) {
			if err := enqueueInvoiceEvent(ctx, tx, tenantID, "invoice.paid.v1", &inv, meta); err != nil {
				return err
			}
		}
		invoice = &inv
		created = true
		return nil
	})
	return invoice, receipt, created, err
}

func (*InvoiceRepository) PaymentReconciliationEventsDurable() bool { return true }

func enqueueInvoiceEvent(ctx context.Context, tx pgx.Tx, tenantID, eventType string, invoice *domain.Invoice, meta map[string]any) error {
	payload, err := json.Marshal(ports.InvoiceEventData(eventType, invoice, meta))
	if err != nil {
		return fmt.Errorf("fees: encode reconciliation event: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fees_outbox (id, tenant_id, event_type, payload, created_at, next_attempt_at)
		VALUES ($1, $2, $3, $4, now(), now())
	`, uuid.NewString(), tenantID, eventType, payload); err != nil {
		return fmt.Errorf("fees: enqueue reconciliation event: %w", err)
	}
	return nil
}

func feesOutboxContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__fees_outbox__"})
}

func (r *InvoiceRepository) ClaimPendingFeeEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := make([]ports.OutboxEvent, 0, limit)
	err := r.db.WithTx(feesOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE fees_outbox
			SET attempts=attempts+1,
			    next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second')
			WHERE id IN (
				SELECT id FROM fees_outbox
				WHERE published_at IS NULL AND next_attempt_at<=now()
				ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1
			)
			RETURNING id, tenant_id, event_type, payload, created_at
		`, limit)
		if err != nil {
			return fmt.Errorf("fees: claim outbox: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item ports.OutboxEvent
			if err := rows.Scan(&item.ID, &item.TenantID, &item.EventType, &item.Payload, &item.CreatedAt); err != nil {
				return fmt.Errorf("fees: scan outbox: %w", err)
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (r *InvoiceRepository) MarkFeeEventPublished(ctx context.Context, id string) error {
	return r.markFeeEvent(ctx, id, "", true)
}

func (r *InvoiceRepository) MarkFeeEventFailed(ctx context.Context, id, message string) error {
	return r.markFeeEvent(ctx, id, message, false)
}

func (r *InvoiceRepository) markFeeEvent(ctx context.Context, id, message string, published bool) error {
	return r.db.WithTx(feesOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if published {
			_, err = tx.Exec(ctx, `UPDATE fees_outbox SET published_at=$2,last_error=NULL WHERE id=$1`, id, time.Now().UTC())
		} else {
			_, err = tx.Exec(ctx, `UPDATE fees_outbox SET last_error=left($2,1000) WHERE id=$1`, id, message)
		}
		return err
	})
}

type scanner interface {
	Scan(dest ...any) error
}

func scanFeeStructure(row scanner) (*domain.FeeStructure, error) {
	var fs domain.FeeStructure
	if err := row.Scan(
		&fs.ID, &fs.TenantID, &fs.Name, &fs.AcademicYearID, &fs.AmountCents, &fs.Currency,
		&fs.Recurrence, &fs.Target, &fs.DueDay, &fs.Description, &fs.Status, &fs.CreatedAt, &fs.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &fs, nil
}

func datePtr(d domain.Date) *time.Time {
	if d.IsEmpty() {
		return nil
	}
	t := d.Time
	return &t
}

func scanInvoice(row scanner) (*domain.Invoice, error) {
	var inv domain.Invoice
	var dueDate *time.Time
	if err := row.Scan(
		&inv.ID, &inv.TenantID, &inv.StudentID, &inv.FeeStructureID, &inv.AmountCents, &inv.BalanceCents,
		&inv.Status, &dueDate, &inv.IssuedAt, &inv.Notes, &inv.CreatedAt, &inv.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if dueDate != nil {
		inv.DueDate = domain.Date{Time: *dueDate}
	}
	return &inv, nil
}

func scanReceipt(row scanner) (*domain.Receipt, error) {
	var receipt domain.Receipt
	if err := row.Scan(
		&receipt.ID, &receipt.TenantID, &receipt.InvoiceID, &receipt.StudentID,
		&receipt.PaymentID, &receipt.AmountCents, &receipt.AppliedCents,
		&receipt.OverpaymentCents, &receipt.Currency, &receipt.ProviderReference,
		&receipt.IssuedAt,
	); err != nil {
		return nil, err
	}
	return &receipt, nil
}
