package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
	"github.com/auraedu/platform/db"
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
	_ ports.FeeStructureRepository = (*FeeStructureRepository)(nil)
	_ ports.InvoiceRepository      = (*InvoiceRepository)(nil)
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
			INSERT INTO fee_structures (id, tenant_id, name, academic_year_id, amount_cents, currency, recurrence, target, due_day, description, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, fs.ID, tenantID, fs.Name, fs.AcademicYearID, fs.AmountCents, fs.Currency, fs.Recurrence, fs.Target, fs.DueDay, fs.Description, fs.Status, fs.CreatedAt, fs.UpdatedAt)
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
			SET name = $3, academic_year_id = $4, amount_cents = $5, currency = $6, recurrence = $7, target = $8, due_day = $9, description = $10, status = $11, updated_at = $12
			WHERE id = $1 AND tenant_id = $2
		`, fs.ID, tenantID, fs.Name, fs.AcademicYearID, fs.AmountCents, fs.Currency, fs.Recurrence, fs.Target, fs.DueDay, fs.Description, fs.Status, fs.UpdatedAt)
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
		_, err := tx.Exec(ctx, `
			INSERT INTO invoices (id, tenant_id, student_id, fee_structure_id, amount_cents, balance_cents, status, due_date, issued_at, notes, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, inv.ID, tenantID, inv.StudentID, inv.FeeStructureID, inv.AmountCents, inv.BalanceCents, inv.Status, datePtr(inv.DueDate), inv.IssuedAt, inv.Notes, inv.CreatedAt, inv.UpdatedAt)
		if err != nil {
			return fmt.Errorf("fees: create invoice: %w", err)
		}
		return nil
	})
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
		_, err := tx.Exec(ctx, `
			UPDATE invoices
			SET student_id = $3, fee_structure_id = $4, amount_cents = $5, balance_cents = $6, status = $7, due_date = $8, issued_at = $9, notes = $10, updated_at = $11
			WHERE id = $1 AND tenant_id = $2
		`, inv.ID, tenantID, inv.StudentID, inv.FeeStructureID, inv.AmountCents, inv.BalanceCents, inv.Status, datePtr(inv.DueDate), inv.IssuedAt, inv.Notes, inv.UpdatedAt)
		if err != nil {
			return fmt.Errorf("fees: update invoice: %w", err)
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
