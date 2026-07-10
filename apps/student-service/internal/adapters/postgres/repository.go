package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/auraedu/platform/db"
	"github.com/auraedu/student-service/internal/domain"
	"github.com/auraedu/student-service/internal/ports"
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

// NewRepository creates a Postgres-backed student repository.
func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

func (r *Repository) Create(ctx context.Context, tenantID string, s *domain.Student) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO students (id, tenant_id, first_name, last_name, student_code, date_of_birth, gender, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, s.ID, tenantID, s.FirstName, s.LastName, s.StudentCode, s.DateOfBirth, s.Gender, s.Status, s.CreatedAt, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("student: create: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.Student, error) {
	var s *domain.Student
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, first_name, last_name, student_code, date_of_birth, gender, status, created_at, updated_at
			FROM students
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanStudent(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("student: get: %w", err)
		}
		s = got
		return nil
	})
	return s, err
}

func (r *Repository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Student, string, error) {
	var out []*domain.Student
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listQuery(ctx, tx, tenantID, limit, cursor)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			s, err := scanStudent(rows)
			if err != nil {
				return err
			}
			out = append(out, s)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("student: list rows: %w", err)
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
			SELECT id, tenant_id, first_name, last_name, student_code, date_of_birth, gender, status, created_at, updated_at
			FROM students
			WHERE tenant_id = $1 AND (created_at, id) > (
				SELECT created_at, id FROM students WHERE id = $2 AND tenant_id = $1
			)
			ORDER BY created_at ASC, id ASC
			LIMIT $3
		`, tenantID, cursor, limit)
	}
	return tx.Query(ctx, `
		SELECT id, tenant_id, first_name, last_name, student_code, date_of_birth, gender, status, created_at, updated_at
		FROM students
		WHERE tenant_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2
	`, tenantID, limit)
}

func (r *Repository) Update(ctx context.Context, tenantID string, s *domain.Student) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE students
			SET first_name = $3, last_name = $4, student_code = $5, date_of_birth = $6,
			    gender = $7, status = $8, updated_at = $9
			WHERE id = $1 AND tenant_id = $2
		`, s.ID, tenantID, s.FirstName, s.LastName, s.StudentCode, s.DateOfBirth, s.Gender, s.Status, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("student: update: %w", err)
		}
		return nil
	})
}

func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM students WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		if err != nil {
			return fmt.Errorf("student: delete: %w", err)
		}
		return nil
	})
}

type scanner interface {
	Scan(dest ...any) error
}

func scanStudent(row scanner) (*domain.Student, error) {
	var s domain.Student
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.FirstName, &s.LastName, &s.StudentCode,
		&s.DateOfBirth, &s.Gender, &s.Status, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}
