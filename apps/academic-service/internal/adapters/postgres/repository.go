package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
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

// NewRepository creates a Postgres-backed academic year repository.
func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

func (r *Repository) Create(ctx context.Context, tenantID string, y *domain.AcademicYear) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO academic_years (id, tenant_id, name, code, start_date, end_date, status, is_current, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, y.ID, tenantID, y.Name, y.Code, y.StartDate, y.EndDate, y.Status, y.IsCurrent, y.CreatedAt, y.UpdatedAt)
		if err != nil {
			return fmt.Errorf("academic: create year: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.AcademicYear, error) {
	var y *domain.AcademicYear
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, name, code, start_date, end_date, status, is_current, created_at, updated_at
			FROM academic_years
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanYear(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("academic: get year: %w", err)
		}
		y = got
		return nil
	})
	return y, err
}

func (r *Repository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.AcademicYear, string, error) {
	var out []*domain.AcademicYear
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listQuery(ctx, tx, tenantID, limit, cursor)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			y, err := scanYear(rows)
			if err != nil {
				return err
			}
			out = append(out, y)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("academic: list rows: %w", err)
		}
		if len(out) == limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listQuery(ctx context.Context, tx pgx.Tx, tenantID string, limit int, cursor string) (pgx.Rows, error) {
	if cursor != "" {
		return tx.Query(ctx, `
			SELECT id, tenant_id, name, code, start_date, end_date, status, is_current, created_at, updated_at
			FROM academic_years
			WHERE tenant_id = $1 AND (created_at, id) > (
				SELECT created_at, id FROM academic_years WHERE id = $2 AND tenant_id = $1
			)
			ORDER BY created_at ASC, id ASC
			LIMIT $3
		`, tenantID, cursor, limit)
	}
	return tx.Query(ctx, `
		SELECT id, tenant_id, name, code, start_date, end_date, status, is_current, created_at, updated_at
		FROM academic_years
		WHERE tenant_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2
	`, tenantID, limit)
}

func (r *Repository) Update(ctx context.Context, tenantID string, y *domain.AcademicYear) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE academic_years
			SET name = $3, code = $4, start_date = $5, end_date = $6, status = $7, is_current = $8, updated_at = $9
			WHERE id = $1 AND tenant_id = $2
		`, y.ID, tenantID, y.Name, y.Code, y.StartDate, y.EndDate, y.Status, y.IsCurrent, y.UpdatedAt)
		if err != nil {
			return fmt.Errorf("academic: update year: %w", err)
		}
		return nil
	})
}

func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM academic_years WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		if err != nil {
			return fmt.Errorf("academic: delete year: %w", err)
		}
		return nil
	})
}

type scanner interface {
	Scan(dest ...any) error
}

func scanYear(row scanner) (*domain.AcademicYear, error) {
	var y domain.AcademicYear
	var start, end time.Time
	if err := row.Scan(
		&y.ID, &y.TenantID, &y.Name, &y.Code, &start, &end,
		&y.Status, &y.IsCurrent, &y.CreatedAt, &y.UpdatedAt,
	); err != nil {
		return nil, err
	}
	y.StartDate = domain.Date{Time: start}
	y.EndDate = domain.Date{Time: end}
	return &y, nil
}
