// Package postgres persists student aggregates in PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

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
			INSERT INTO students (id, tenant_id, first_name, last_name, student_code, date_of_birth, gender, status, class_id, academic_year_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, s.ID, tenantID, s.FirstName, s.LastName, s.StudentCode, s.DateOfBirth, s.Gender, s.Status, s.ClassID, s.AcademicYearID, s.CreatedAt, s.UpdatedAt)
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
			SELECT id, tenant_id, first_name, last_name, student_code, date_of_birth, gender, status, class_id, academic_year_id, created_at, updated_at
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

func (r *Repository) List(ctx context.Context, tenantID string, classID *string, limit int, cursor string) ([]*domain.Student, string, error) {
	var out []*domain.Student
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listQuery(ctx, tx, tenantID, classID, limit, cursor)
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

func listQuery(ctx context.Context, tx pgx.Tx, tenantID string, classID *string, limit int, cursor string) (pgx.Rows, error) {
	// ($n::uuid IS NULL OR class_id = $n) keeps one plan for the filtered (roster) and
	// unfiltered cases: a NULL filter parameter disables the class predicate.
	if cursor != "" {
		return tx.Query(ctx, `
			SELECT id, tenant_id, first_name, last_name, student_code, date_of_birth, gender, status, class_id, academic_year_id, created_at, updated_at
			FROM students
			WHERE tenant_id = $1 AND ($4::uuid IS NULL OR class_id = $4::uuid) AND (created_at, id) > (
				SELECT created_at, id FROM students WHERE id = $2 AND tenant_id = $1
			)
			ORDER BY created_at ASC, id ASC
			LIMIT $3
		`, tenantID, cursor, limit, classID)
	}
	return tx.Query(ctx, `
		SELECT id, tenant_id, first_name, last_name, student_code, date_of_birth, gender, status, class_id, academic_year_id, created_at, updated_at
		FROM students
		WHERE tenant_id = $1 AND ($3::uuid IS NULL OR class_id = $3::uuid)
		ORDER BY created_at ASC, id ASC
		LIMIT $2
	`, tenantID, limit, classID)
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
	var dob *time.Time
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.FirstName, &s.LastName, &s.StudentCode,
		&dob, &s.Gender, &s.Status, &s.ClassID, &s.AcademicYearID, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if dob != nil {
		v := dob.Format(time.DateOnly)
		s.DateOfBirth = &v
	}
	return &s, nil
}

// --- Guardians ---

func (r *Repository) CreateGuardian(ctx context.Context, tenantID string, g *domain.Guardian) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO guardians (id, tenant_id, first_name, last_name, relationship, phone, email, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, g.ID, tenantID, g.FirstName, g.LastName, g.Relationship, g.Phone, g.Email, g.CreatedAt, g.UpdatedAt)
		if err != nil {
			return fmt.Errorf("student: create guardian: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetGuardianByID(ctx context.Context, tenantID, id string) (*domain.Guardian, error) {
	var g *domain.Guardian
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, first_name, last_name, relationship, phone, email, created_at, updated_at
			FROM guardians
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanGuardian(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("student: get guardian: %w", err)
		}
		g = got
		return nil
	})
	return g, err
}

func (r *Repository) ListGuardiansByStudent(ctx context.Context, tenantID, studentID string, limit int, _ string) ([]*domain.Guardian, string, error) {
	var out []*domain.Guardian
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT g.id, g.tenant_id, g.first_name, g.last_name, g.relationship, g.phone, g.email, g.created_at, g.updated_at
			FROM guardians g
			JOIN student_guardians sg ON sg.guardian_id = g.id AND sg.tenant_id = g.tenant_id
			WHERE g.tenant_id = $1 AND sg.student_id = $2
			ORDER BY g.created_at ASC, g.id ASC
			LIMIT $3
		`, tenantID, studentID, limit)
		if err != nil {
			return fmt.Errorf("student: list guardians: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			g, err := scanGuardian(rows)
			if err != nil {
				return err
			}
			out = append(out, g)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("student: list guardians rows: %w", err)
		}
		if len(out) == limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func (r *Repository) UpdateGuardian(ctx context.Context, tenantID string, g *domain.Guardian) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE guardians
			SET first_name = $3, last_name = $4, relationship = $5, phone = $6, email = $7, updated_at = $8
			WHERE id = $1 AND tenant_id = $2
		`, g.ID, tenantID, g.FirstName, g.LastName, g.Relationship, g.Phone, g.Email, g.UpdatedAt)
		if err != nil {
			return fmt.Errorf("student: update guardian: %w", err)
		}
		return nil
	})
}

func (r *Repository) DeleteGuardian(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM guardians WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		if err != nil {
			return fmt.Errorf("student: delete guardian: %w", err)
		}
		return nil
	})
}

// --- Student ↔ Guardian links ---

func (r *Repository) LinkGuardianToStudent(ctx context.Context, tenantID string, link *domain.StudentGuardian) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO student_guardians (id, tenant_id, student_id, guardian_id, relationship, is_primary, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (tenant_id, student_id, guardian_id) DO UPDATE SET
				relationship = EXCLUDED.relationship,
				is_primary = EXCLUDED.is_primary
		`, link.ID, tenantID, link.StudentID, link.GuardianID, link.Relationship, link.IsPrimary, link.CreatedAt)
		if err != nil {
			return fmt.Errorf("student: link guardian: %w", err)
		}
		return nil
	})
}

func (r *Repository) UnlinkGuardianFromStudent(ctx context.Context, tenantID, studentID, guardianID string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			DELETE FROM student_guardians
			WHERE tenant_id = $1 AND student_id = $2 AND guardian_id = $3
		`, tenantID, studentID, guardianID)
		if err != nil {
			return fmt.Errorf("student: unlink guardian: %w", err)
		}
		return nil
	})
}

func scanGuardian(row scanner) (*domain.Guardian, error) {
	var g domain.Guardian
	if err := row.Scan(
		&g.ID, &g.TenantID, &g.FirstName, &g.LastName, &g.Relationship,
		&g.Phone, &g.Email, &g.CreatedAt, &g.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &g, nil
}
