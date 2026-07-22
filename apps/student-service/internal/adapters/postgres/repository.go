// Package postgres persists student aggregates in PostgreSQL.
//
//nolint:lll // SQL column lists intentionally mirror their scan order for auditability.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/student-service/internal/domain"
	"github.com/auraedu/student-service/internal/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Repository is the Postgres implementation of ports.Repository.
// It uses platform/db.WithTx so that app.tenant_id is set on the same connection
// that executes the query, which makes the Row-Level Security policy effective.
// Every query also filters by tenant_id explicitly as defense-in-depth.
type Repository struct {
	db *db.DB
}

var (
	_ ports.Repository          = (*Repository)(nil)
	_ ports.LifecycleRepository = (*Repository)(nil)
	_ ports.OutboxRepository    = (*Repository)(nil)
)

// NewRepository creates a Postgres-backed student repository.
func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

func (r *Repository) Create(ctx context.Context, tenantID string, s *domain.Student) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO students (id, tenant_id, first_name, last_name, student_code, date_of_birth, gender, status, class_id, academic_year_id, user_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, s.ID, tenantID, s.FirstName, s.LastName, s.StudentCode, s.DateOfBirth, s.Gender, s.Status, s.ClassID, s.AcademicYearID, s.UserID, s.CreatedAt, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("student: create: %w", err)
		}
		if s.ClassID != nil && s.AcademicYearID != nil {
			enrollment, err := domain.NewEnrollment(tenantID, s.ID, *s.ClassID, *s.AcademicYearID, s.CreatedAt)
			if err != nil {
				return err
			}
			if err := insertEnrollment(ctx, tx, enrollment); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) CreateEnrollment(ctx context.Context, tenantID string, enrollment *domain.Enrollment) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := insertEnrollment(ctx, tx, enrollment); err != nil {
			return err
		}
		result, err := tx.Exec(ctx, `
			UPDATE students SET class_id = $3, academic_year_id = $4, updated_at = $5
			WHERE tenant_id = $1 AND id = $2
		`, tenantID, enrollment.StudentID, enrollment.ClassID, enrollment.AcademicYearID, enrollment.EnrolledAt)
		if err != nil {
			return fmt.Errorf("student: update current enrollment: %w", err)
		}
		if result.RowsAffected() != 1 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func insertEnrollment(ctx context.Context, tx pgx.Tx, enrollment *domain.Enrollment) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO enrollments (id, tenant_id, student_id, class_id, academic_year_id, enrolled_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, enrollment.ID, enrollment.TenantID, enrollment.StudentID, enrollment.ClassID, enrollment.AcademicYearID, enrollment.EnrolledAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: student already has an enrollment for this academic year", domain.ErrConflict)
		}
		return fmt.Errorf("student: create enrollment: %w", err)
	}
	return nil
}

func (r *Repository) ListEnrollments(ctx context.Context, tenantID, studentID string, limit int, cursor string) ([]*domain.Enrollment, string, error) {
	var out []*domain.Enrollment
	var next string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		args := []any{tenantID, studentID, limit}
		where := "tenant_id = $1 AND student_id = $2"
		if cursor != "" {
			args = append(args, cursor)
			where += " AND (enrolled_at, id) > (SELECT enrolled_at, id FROM enrollments WHERE tenant_id = $1 AND id = $4)"
		}
		rows, err := tx.Query(ctx, `SELECT id, tenant_id, student_id, class_id, academic_year_id, enrolled_at
			FROM enrollments WHERE `+where+` ORDER BY enrolled_at ASC, id ASC LIMIT $3`, args...)
		if err != nil {
			return fmt.Errorf("student: list enrollments: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var enrollment domain.Enrollment
			if err := rows.Scan(&enrollment.ID, &enrollment.TenantID, &enrollment.StudentID, &enrollment.ClassID, &enrollment.AcademicYearID, &enrollment.EnrolledAt); err != nil {
				return fmt.Errorf("student: scan enrollment: %w", err)
			}
			out = append(out, &enrollment)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(out) == limit && len(out) > 0 {
			next = out[len(out)-1].ID
		}
		return nil
	})
	return out, next, err
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.Student, error) {
	var s *domain.Student
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, first_name, last_name, student_code, date_of_birth, gender, status, class_id, academic_year_id, user_id, created_at, updated_at
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

func (r *Repository) GetStudentByUserID(ctx context.Context, tenantID, userID string) (*domain.Student, error) {
	var s *domain.Student
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, first_name, last_name, student_code, date_of_birth, gender, status, class_id, academic_year_id, user_id, created_at, updated_at
			FROM students
			WHERE user_id = $1 AND tenant_id = $2
		`, userID, tenantID)
		got, err := scanStudent(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("student: get by user: %w", err)
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

func (r *Repository) ListStudentIDsByClassIDs(ctx context.Context, tenantID string, classIDs []string) ([]string, error) {
	if len(classIDs) == 0 {
		return []string{}, nil
	}
	var ids []string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id FROM students
			WHERE tenant_id = $1 AND class_id = ANY($2) AND status = 'active'
			ORDER BY id
		`, tenantID, classIDs)
		if err != nil {
			return fmt.Errorf("student: list ids by classes: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("student: scan class roster id: %w", err)
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

func listQuery(ctx context.Context, tx pgx.Tx, tenantID string, classID *string, limit int, cursor string) (pgx.Rows, error) {
	// ($n::uuid IS NULL OR class_id = $n) keeps one plan for the filtered (roster) and
	// unfiltered cases: a NULL filter parameter disables the class predicate.
	if cursor != "" {
		return tx.Query(ctx, `
			SELECT id, tenant_id, first_name, last_name, student_code, date_of_birth, gender, status, class_id, academic_year_id, user_id, created_at, updated_at
			FROM students
			WHERE tenant_id = $1 AND ($4::uuid IS NULL OR class_id = $4::uuid) AND (created_at, id) > (
				SELECT created_at, id FROM students WHERE id = $2 AND tenant_id = $1
			)
			ORDER BY created_at ASC, id ASC
			LIMIT $3
		`, tenantID, cursor, limit, classID)
	}
	return tx.Query(ctx, `
		SELECT id, tenant_id, first_name, last_name, student_code, date_of_birth, gender, status, class_id, academic_year_id, user_id, created_at, updated_at
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
			    gender = $7, status = $8, user_id = $9, updated_at = $10
			WHERE id = $1 AND tenant_id = $2
		`, s.ID, tenantID, s.FirstName, s.LastName, s.StudentCode, s.DateOfBirth, s.Gender, s.Status, s.UserID, s.UpdatedAt)
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

// CommitStudentLifecycle persists one public lifecycle boundary and its event atomically.
func (r *Repository) CommitStudentLifecycle(ctx context.Context, tenantID string, mutation ports.LifecycleMutation, eventType string, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("student: encode lifecycle event: %w", err)
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := applyLifecycleMutation(ctx, tx, tenantID, mutation); err != nil {
			return fmt.Errorf("student: lifecycle mutation: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO student_outbox (id,tenant_id,event_type,payload) VALUES ($1,$2,$3,$4)`, uuid.NewString(), tenantID, eventType, encoded); err != nil {
			return fmt.Errorf("student: enqueue lifecycle event: %w", err)
		}
		return nil
	})
}

func applyLifecycleMutation(
	ctx context.Context,
	tx pgx.Tx,
	tenantID string,
	mutation ports.LifecycleMutation,
) error {
	switch mutation.Kind {
	case ports.MutationStudentCreate:
		return createStudentLifecycle(ctx, tx, tenantID, mutation.Student)
	case ports.MutationStudentUpdate:
		return updateStudentLifecycle(ctx, tx, tenantID, mutation.Student)
	case ports.MutationStudentDelete:
		return deleteStudentLifecycle(ctx, tx, tenantID, mutation.Student.ID)
	case ports.MutationEnrollmentCreate:
		return createEnrollmentLifecycle(ctx, tx, tenantID, mutation.Enrollment)
	case ports.MutationGuardianCreate:
		return createGuardianLifecycle(ctx, tx, tenantID, mutation.Guardian)
	case ports.MutationGuardianUpdate:
		return updateGuardianLifecycle(ctx, tx, tenantID, mutation.Guardian)
	case ports.MutationGuardianDelete:
		return deleteGuardianLifecycle(ctx, tx, tenantID, mutation.Guardian.ID)
	case ports.MutationGuardianLink:
		return linkGuardianLifecycle(ctx, tx, tenantID, mutation.Link)
	case ports.MutationGuardianUnlink:
		return unlinkGuardianLifecycle(ctx, tx, tenantID, mutation.StudentID, mutation.GuardianID)
	default:
		return fmt.Errorf("student: unsupported lifecycle mutation %q", mutation.Kind)
	}
}

func createStudentLifecycle(ctx context.Context, tx pgx.Tx, tenantID string, student *domain.Student) error {
	_, err := tx.Exec(ctx, `INSERT INTO students (id,tenant_id,first_name,last_name,student_code,date_of_birth,gender,status,class_id,academic_year_id,user_id,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, student.ID, tenantID, student.FirstName, student.LastName, student.StudentCode, student.DateOfBirth, student.Gender, student.Status, student.ClassID, student.AcademicYearID, student.UserID, student.CreatedAt, student.UpdatedAt)
	if err != nil || student.ClassID == nil || student.AcademicYearID == nil {
		return err
	}
	enrollment, err := domain.NewEnrollment(
		tenantID,
		student.ID,
		*student.ClassID,
		*student.AcademicYearID,
		student.CreatedAt,
	)
	if err != nil {
		return err
	}
	return insertEnrollment(ctx, tx, enrollment)
}

func updateStudentLifecycle(ctx context.Context, tx pgx.Tx, tenantID string, student *domain.Student) error {
	tag, err := tx.Exec(ctx, `UPDATE students SET first_name=$3,last_name=$4,student_code=$5,date_of_birth=$6,gender=$7,status=$8,user_id=$9,updated_at=$10 WHERE id=$1 AND tenant_id=$2`, student.ID, tenantID, student.FirstName, student.LastName, student.StudentCode, student.DateOfBirth, student.Gender, student.Status, student.UserID, student.UpdatedAt)
	return oneRowOrError(tag, err)
}

func deleteStudentLifecycle(ctx context.Context, tx pgx.Tx, tenantID, studentID string) error {
	tag, err := tx.Exec(ctx, `DELETE FROM students WHERE id=$1 AND tenant_id=$2`, studentID, tenantID)
	return oneRowOrError(tag, err)
}

func createEnrollmentLifecycle(
	ctx context.Context,
	tx pgx.Tx,
	tenantID string,
	enrollment *domain.Enrollment,
) error {
	if err := insertEnrollment(ctx, tx, enrollment); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE students SET class_id=$3,academic_year_id=$4,updated_at=$5 WHERE tenant_id=$1 AND id=$2`, tenantID, enrollment.StudentID, enrollment.ClassID, enrollment.AcademicYearID, enrollment.EnrolledAt)
	return oneRowOrError(tag, err)
}

func createGuardianLifecycle(ctx context.Context, tx pgx.Tx, tenantID string, guardian *domain.Guardian) error {
	_, err := tx.Exec(ctx, `INSERT INTO guardians (id,tenant_id,first_name,last_name,relationship,phone,email,user_id,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, guardian.ID, tenantID, guardian.FirstName, guardian.LastName, guardian.Relationship, guardian.Phone, guardian.Email, guardian.UserID, guardian.CreatedAt, guardian.UpdatedAt)
	return err
}

func updateGuardianLifecycle(ctx context.Context, tx pgx.Tx, tenantID string, guardian *domain.Guardian) error {
	tag, err := tx.Exec(ctx, `UPDATE guardians SET first_name=$3,last_name=$4,relationship=$5,phone=$6,email=$7,updated_at=$8 WHERE id=$1 AND tenant_id=$2`, guardian.ID, tenantID, guardian.FirstName, guardian.LastName, guardian.Relationship, guardian.Phone, guardian.Email, guardian.UpdatedAt)
	return oneRowOrError(tag, err)
}

func deleteGuardianLifecycle(ctx context.Context, tx pgx.Tx, tenantID, guardianID string) error {
	tag, err := tx.Exec(ctx, `DELETE FROM guardians WHERE id=$1 AND tenant_id=$2`, guardianID, tenantID)
	return oneRowOrError(tag, err)
}

func linkGuardianLifecycle(
	ctx context.Context,
	tx pgx.Tx,
	tenantID string,
	link *domain.StudentGuardian,
) error {
	_, err := tx.Exec(ctx, `INSERT INTO student_guardians (id,tenant_id,student_id,guardian_id,relationship,is_primary,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (tenant_id,student_id,guardian_id) DO UPDATE SET relationship=EXCLUDED.relationship,is_primary=EXCLUDED.is_primary`, link.ID, tenantID, link.StudentID, link.GuardianID, link.Relationship, link.IsPrimary, link.CreatedAt)
	return err
}

func unlinkGuardianLifecycle(
	ctx context.Context,
	tx pgx.Tx,
	tenantID string,
	studentID string,
	guardianID string,
) error {
	tag, err := tx.Exec(ctx, `DELETE FROM student_guardians WHERE tenant_id=$1 AND student_id=$2 AND guardian_id=$3`, tenantID, studentID, guardianID)
	return oneRowOrError(tag, err)
}

func oneRowOrError(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func studentOutboxContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__student_outbox__"})
}

func (r *Repository) ClaimPendingStudentEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := make([]ports.OutboxEvent, 0, limit)
	err := r.db.WithTx(studentOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `UPDATE student_outbox SET attempts=attempts+1,next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second') WHERE id IN (SELECT id FROM student_outbox WHERE published_at IS NULL AND next_attempt_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1) RETURNING id::text,tenant_id,event_type,payload`, limit)
		if err != nil {
			return fmt.Errorf("student: claim outbox: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item ports.OutboxEvent
			if err := rows.Scan(&item.ID, &item.TenantID, &item.EventType, &item.Payload); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (r *Repository) MarkStudentEventPublished(ctx context.Context, id string) error {
	return r.db.WithTx(studentOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE student_outbox SET published_at=now(),last_error=NULL WHERE id=$1`, id)
		return err
	})
}

func (r *Repository) MarkStudentEventFailed(ctx context.Context, id, message string) error {
	return r.db.WithTx(studentOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE student_outbox SET last_error=left($2,1000) WHERE id=$1`, id, message)
		return err
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
		&dob, &s.Gender, &s.Status, &s.ClassID, &s.AcademicYearID, &s.UserID, &s.CreatedAt, &s.UpdatedAt,
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
			INSERT INTO guardians (id, tenant_id, first_name, last_name, relationship, phone, email, user_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, g.ID, tenantID, g.FirstName, g.LastName, g.Relationship, g.Phone, g.Email, g.UserID, g.CreatedAt, g.UpdatedAt)
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
			SELECT id, tenant_id, first_name, last_name, relationship, phone, email, user_id, created_at, updated_at
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

func (r *Repository) GetGuardianByUserID(ctx context.Context, tenantID, userID string) (*domain.Guardian, error) {
	var g *domain.Guardian
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, first_name, last_name, relationship, phone, email, user_id, created_at, updated_at
			FROM guardians
			WHERE user_id = $1 AND tenant_id = $2
		`, userID, tenantID)
		got, err := scanGuardian(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("student: get guardian by user: %w", err)
		}
		g = got
		return nil
	})
	return g, err
}

func (r *Repository) ListStudentsByGuardian(ctx context.Context, tenantID, guardianID string) ([]*domain.Student, error) {
	var out []*domain.Student
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT s.id, s.tenant_id, s.first_name, s.last_name, s.student_code, s.date_of_birth, s.gender, s.status, s.class_id, s.academic_year_id, s.user_id, s.created_at, s.updated_at
			FROM students s
			JOIN student_guardians sg ON sg.student_id = s.id AND sg.tenant_id = s.tenant_id
			WHERE s.tenant_id = $1 AND sg.guardian_id = $2
			ORDER BY s.created_at ASC, s.id ASC
		`, tenantID, guardianID)
		if err != nil {
			return fmt.Errorf("student: list students by guardian: %w", err)
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
			return fmt.Errorf("student: list students by guardian rows: %w", err)
		}
		return nil
	})
	return out, err
}

func (r *Repository) ListGuardiansByStudent(ctx context.Context, tenantID, studentID string, limit int, _ string) ([]*domain.Guardian, string, error) {
	var out []*domain.Guardian
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT g.id, g.tenant_id, g.first_name, g.last_name, g.relationship, g.phone, g.email, g.user_id, g.created_at, g.updated_at
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
		&g.Phone, &g.Email, &g.UserID, &g.CreatedAt, &g.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &g, nil
}
