package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/auraedu/platform/db"
	"github.com/auraedu/staff-service/internal/domain"
	"github.com/auraedu/staff-service/internal/ports"
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

// NewRepository creates a Postgres-backed staff repository.
func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

func (r *Repository) Create(ctx context.Context, tenantID string, s *domain.Staff) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO staff (id, tenant_id, first_name, last_name, staff_type, email, staff_code, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, s.ID, tenantID, s.FirstName, s.LastName, s.StaffType, s.Email, s.StaffCode, s.Status, s.CreatedAt, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("staff: create: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.Staff, error) {
	var s *domain.Staff
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, first_name, last_name, staff_type, email, staff_code, status, created_at, updated_at
			FROM staff
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanStaff(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("staff: get: %w", err)
		}
		s = got
		return nil
	})
	return s, err
}

func (r *Repository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Staff, string, error) {
	var out []*domain.Staff
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listQuery(ctx, tx, tenantID, limit, cursor)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			s, err := scanStaff(rows)
			if err != nil {
				return err
			}
			out = append(out, s)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("staff: list rows: %w", err)
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
			SELECT id, tenant_id, first_name, last_name, staff_type, email, staff_code, status, created_at, updated_at
			FROM staff
			WHERE tenant_id = $1 AND (created_at, id) > (
				SELECT created_at, id FROM staff WHERE id = $2 AND tenant_id = $1
			)
			ORDER BY created_at ASC, id ASC
			LIMIT $3
		`, tenantID, cursor, limit)
	}
	return tx.Query(ctx, `
		SELECT id, tenant_id, first_name, last_name, staff_type, email, staff_code, status, created_at, updated_at
		FROM staff
		WHERE tenant_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2
	`, tenantID, limit)
}

func (r *Repository) Update(ctx context.Context, tenantID string, s *domain.Staff) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE staff
			SET first_name = $3, last_name = $4, staff_type = $5, email = $6,
			    staff_code = $7, status = $8, updated_at = $9
			WHERE id = $1 AND tenant_id = $2
		`, s.ID, tenantID, s.FirstName, s.LastName, s.StaffType, s.Email, s.StaffCode, s.Status, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("staff: update: %w", err)
		}
		return nil
	})
}

func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM staff WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		if err != nil {
			return fmt.Errorf("staff: delete: %w", err)
		}
		return nil
	})
}

type scanner interface {
	Scan(dest ...any) error
}

func scanStaff(row scanner) (*domain.Staff, error) {
	var s domain.Staff
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.FirstName, &s.LastName, &s.StaffType,
		&s.Email, &s.StaffCode, &s.Status, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}
