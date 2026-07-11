// Package postgres provides the Postgres implementation of the attendance repository port.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
)

// Repository is the Postgres implementation of ports.Repository.
// It uses platform/db.WithTx so that app.tenant_id is set on the same connection
// that executes the query, which makes the Row-Level Security policy effective.
// Every query also filters by tenant_id explicitly as defense-in-depth.
type Repository struct {
	db *db.DB
}

var _ ports.Repository = (*Repository)(nil)

// NewRepository creates a Postgres-backed attendance repository.
func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

func (r *Repository) Create(ctx context.Context, tenantID string, rec *domain.AttendanceRecord) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO attendance_records (id, tenant_id, student_id, academic_year_id, date, status, reason, marked_by, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, rec.ID, tenantID, rec.StudentID, rec.AcademicYearID, rec.Date, rec.Status, rec.Reason, rec.MarkedBy, rec.CreatedAt, rec.UpdatedAt)
		if err != nil {
			return fmt.Errorf("attendance: create record: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.AttendanceRecord, error) {
	var rec *domain.AttendanceRecord
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, student_id, academic_year_id, date, status, reason, marked_by, created_at, updated_at
			FROM attendance_records
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, id, tenantID)
		got, err := scanRecord(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("attendance: get record: %w", err)
		}
		rec = got
		return nil
	})
	return rec, err
}

func (r *Repository) List(ctx context.Context, tenantID string, filter ports.ListFilter) ([]*domain.AttendanceRecord, string, error) {
	var out []*domain.AttendanceRecord
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listQuery(ctx, tx, tenantID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			rec, err := scanRecord(rows)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("attendance: list rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listQuery(ctx context.Context, tx pgx.Tx, tenantID string, filter ports.ListFilter) (pgx.Rows, error) {
	args := []any{tenantID}
	where := "tenant_id = $1 AND deleted_at IS NULL"

	if filter.StudentID != "" {
		args = append(args, filter.StudentID)
		where += fmt.Sprintf(" AND student_id = $%d", len(args))
	}
	if filter.AcademicYearID != "" {
		args = append(args, filter.AcademicYearID)
		where += fmt.Sprintf(" AND academic_year_id = $%d", len(args))
	}
	if filter.Date != "" {
		args = append(args, filter.Date)
		where += fmt.Sprintf(" AND date = $%d", len(args))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM attendance_records WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, student_id, academic_year_id, date, status, reason, marked_by, created_at, updated_at
		FROM attendance_records
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *Repository) Update(ctx context.Context, tenantID string, rec *domain.AttendanceRecord) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE attendance_records
			SET student_id = $3, academic_year_id = $4, date = $5, status = $6, reason = $7, marked_by = $8, updated_at = $9
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, rec.ID, tenantID, rec.StudentID, rec.AcademicYearID, rec.Date, rec.Status, rec.Reason, rec.MarkedBy, rec.UpdatedAt)
		if err != nil {
			return fmt.Errorf("attendance: update record: %w", err)
		}
		return nil
	})
}

func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		now := time.Now().UTC()
		_, err := tx.Exec(ctx, `
			UPDATE attendance_records
			SET deleted_at = $3
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, id, tenantID, now)
		if err != nil {
			return fmt.Errorf("attendance: delete record: %w", err)
		}
		return nil
	})
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRecord(row scanner) (*domain.AttendanceRecord, error) {
	var rec domain.AttendanceRecord
	var date time.Time
	if err := row.Scan(
		&rec.ID, &rec.TenantID, &rec.StudentID, &rec.AcademicYearID, &date,
		&rec.Status, &rec.Reason, &rec.MarkedBy, &rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return nil, err
	}
	rec.Date = domain.Date{Time: date}
	return &rec, nil
}
