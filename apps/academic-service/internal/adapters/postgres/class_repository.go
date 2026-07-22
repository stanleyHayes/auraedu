package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
)

// ClassRepository is the Postgres implementation of ports.ClassRepository.
// It uses platform/db.WithTx so that app.tenant_id is set on the same connection
// that executes the query, which makes the Row-Level Security policy effective.
// Every query also filters by tenant_id explicitly as defense-in-depth.
type ClassRepository struct {
	db *db.DB
}

var _ ports.ClassRepository = (*ClassRepository)(nil)

// NewClassRepository creates a Postgres-backed class repository.
func NewClassRepository(database *db.DB) *ClassRepository { return &ClassRepository{db: database} }

func (r *ClassRepository) Create(ctx context.Context, tenantID string, c *domain.Class) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO classes (id, tenant_id, name, academic_year_id, class_teacher_id, capacity, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, c.ID, tenantID, c.Name, c.AcademicYearID, c.ClassTeacherID, c.Capacity, c.CreatedAt, c.UpdatedAt)
		if err != nil {
			return fmt.Errorf("academic: create class: %w", err)
		}
		return nil
	})
}

func (r *ClassRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Class, error) {
	var c *domain.Class
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, name, academic_year_id, class_teacher_id, capacity, created_at, updated_at
			FROM classes
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanClass(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("academic: get class: %w", err)
		}
		c = got
		return nil
	})
	return c, err
}

func (r *ClassRepository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Class, string, error) {
	var out []*domain.Class
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listClassesQuery(ctx, tx, tenantID, limit, cursor)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			c, err := scanClass(rows)
			if err != nil {
				return err
			}
			out = append(out, c)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("academic: list class rows: %w", err)
		}
		if len(out) == limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func (r *ClassRepository) ListIDsByTeacher(ctx context.Context, tenantID, staffID string) ([]string, error) {
	var ids []string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id FROM classes
			WHERE tenant_id = $1 AND class_teacher_id = $2
			ORDER BY id
		`, tenantID, staffID)
		if err != nil {
			return fmt.Errorf("academic: list teacher class ids: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("academic: scan teacher class id: %w", err)
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	if ids == nil {
		ids = []string{}
	}
	return ids, err
}

func listClassesQuery(ctx context.Context, tx pgx.Tx, tenantID string, limit int, cursor string) (pgx.Rows, error) {
	if cursor != "" {
		return tx.Query(ctx, `
			SELECT id, tenant_id, name, academic_year_id, class_teacher_id, capacity, created_at, updated_at
			FROM classes
			WHERE tenant_id = $1 AND (created_at, id) > (
				SELECT created_at, id FROM classes WHERE id = $2 AND tenant_id = $1
			)
			ORDER BY created_at ASC, id ASC
			LIMIT $3
		`, tenantID, cursor, limit)
	}
	return tx.Query(ctx, `
		SELECT id, tenant_id, name, academic_year_id, class_teacher_id, capacity, created_at, updated_at
		FROM classes
		WHERE tenant_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2
	`, tenantID, limit)
}

func (r *ClassRepository) Update(ctx context.Context, tenantID string, c *domain.Class) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE classes
			SET name = $3, class_teacher_id = $4, capacity = $5, updated_at = $6
			WHERE id = $1 AND tenant_id = $2
		`, c.ID, tenantID, c.Name, c.ClassTeacherID, c.Capacity, c.UpdatedAt)
		if err != nil {
			return fmt.Errorf("academic: update class: %w", err)
		}
		return nil
	})
}

func (r *ClassRepository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM classes WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		if err != nil {
			return fmt.Errorf("academic: delete class: %w", err)
		}
		return nil
	})
}

func scanClass(row scanner) (*domain.Class, error) {
	var c domain.Class
	if err := row.Scan(
		&c.ID, &c.TenantID, &c.Name, &c.AcademicYearID, &c.ClassTeacherID, &c.Capacity,
		&c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &c, nil
}
