// Package postgres persists staff aggregates in PostgreSQL.
//
//nolint:lll // SQL column lists intentionally mirror their scan order for auditability.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/staff-service/internal/domain"
	"github.com/auraedu/staff-service/internal/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository is the Postgres implementation of ports.Repository.
// It uses platform/db.WithTx so that app.tenant_id is set on the same connection
// that executes the query, which makes the Row-Level Security policy effective.
// Every query also filters by tenant_id explicitly as defense-in-depth.
type Repository struct {
	db *db.DB
}

var (
	_ ports.Repository           = (*Repository)(nil)
	_ ports.LifecycleRepository  = (*Repository)(nil)
	_ ports.OutboxRepository     = (*Repository)(nil)
	_ ports.AssignmentRepository = (*Repository)(nil)
)

// NewRepository creates a Postgres-backed staff repository.
func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

func (r *Repository) Create(ctx context.Context, tenantID string, s *domain.Staff) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return insertStaff(ctx, tx, tenantID, s)
	})
}

func insertStaff(ctx context.Context, tx pgx.Tx, tenantID string, s *domain.Staff) error {
	_, err := tx.Exec(ctx, `
			INSERT INTO staff (id, tenant_id, first_name, last_name, staff_type, email, user_id, staff_code, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, s.ID, tenantID, s.FirstName, s.LastName, s.StaffType, s.Email, s.UserID, s.StaffCode, s.Status, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("staff: create: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.Staff, error) {
	var s *domain.Staff
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, first_name, last_name, staff_type, email, user_id, staff_code, status, created_at, updated_at
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

func (r *Repository) GetByUserID(ctx context.Context, tenantID, userID string) (*domain.Staff, error) {
	var s *domain.Staff
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		got, err := scanStaff(tx.QueryRow(ctx, `
			SELECT id, tenant_id, first_name, last_name, staff_type, email, user_id, staff_code, status, created_at, updated_at
			FROM staff WHERE tenant_id = $1 AND user_id = $2
		`, tenantID, userID))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("staff: get by user id: %w", err)
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
			SELECT id, tenant_id, first_name, last_name, staff_type, email, user_id, staff_code, status, created_at, updated_at
			FROM staff
			WHERE tenant_id = $1 AND (created_at, id) > (
				SELECT created_at, id FROM staff WHERE id = $2 AND tenant_id = $1
			)
			ORDER BY created_at ASC, id ASC
			LIMIT $3
		`, tenantID, cursor, limit)
	}
	return tx.Query(ctx, `
		SELECT id, tenant_id, first_name, last_name, staff_type, email, user_id, staff_code, status, created_at, updated_at
		FROM staff
		WHERE tenant_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2
	`, tenantID, limit)
}

func (r *Repository) Update(ctx context.Context, tenantID string, s *domain.Staff) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return updateStaff(ctx, tx, tenantID, s)
	})
}

func updateStaff(ctx context.Context, tx pgx.Tx, tenantID string, s *domain.Staff) error {
	tag, err := tx.Exec(ctx, `
			UPDATE staff
			SET first_name = $3, last_name = $4, staff_type = $5, email = $6, user_id = $7,
			    staff_code = $8, status = $9, updated_at = $10
			WHERE id = $1 AND tenant_id = $2
		`, s.ID, tenantID, s.FirstName, s.LastName, s.StaffType, s.Email, s.UserID, s.StaffCode, s.Status, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("staff: update: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return nil
}

// CommitStaffLifecycle persists the aggregate mutation and event in one transaction.
func (r *Repository) CommitStaffLifecycle(ctx context.Context, tenantID string, staff *domain.Staff, mutation, eventType string, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("staff: encode lifecycle event: %w", err)
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		switch mutation {
		case ports.StaffMutationCreate:
			if err := insertStaff(ctx, tx, tenantID, staff); err != nil {
				return err
			}
		case ports.StaffMutationUpdate:
			if err := updateStaff(ctx, tx, tenantID, staff); err != nil {
				return err
			}
		case ports.StaffMutationDelete:
			tag, err := tx.Exec(ctx, `DELETE FROM staff WHERE id=$1 AND tenant_id=$2`, staff.ID, tenantID)
			if err != nil {
				return fmt.Errorf("staff: delete: %w", err)
			}
			if tag.RowsAffected() != 1 {
				return domain.ErrNotFound
			}
		default:
			return fmt.Errorf("staff: unsupported lifecycle mutation %q", mutation)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO staff_outbox (id,tenant_id,event_type,payload) VALUES ($1,$2,$3,$4)`, uuid.NewString(), tenantID, eventType, encoded); err != nil {
			return fmt.Errorf("staff: enqueue lifecycle event: %w", err)
		}
		return nil
	})
}

func staffOutboxContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__staff_outbox__"})
}

func (r *Repository) ClaimPendingStaffEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := make([]ports.OutboxEvent, 0, limit)
	err := r.db.WithTx(staffOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `UPDATE staff_outbox SET attempts=attempts+1,next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second') WHERE id IN (SELECT id FROM staff_outbox WHERE published_at IS NULL AND next_attempt_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1) RETURNING id::text,tenant_id,event_type,payload`, limit)
		if err != nil {
			return fmt.Errorf("staff: claim outbox: %w", err)
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

func (r *Repository) MarkStaffEventPublished(ctx context.Context, id string) error {
	return r.db.WithTx(staffOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE staff_outbox SET published_at=now(),last_error=NULL WHERE id=$1`, id)
		return err
	})
}

func (r *Repository) MarkStaffEventFailed(ctx context.Context, id, message string) error {
	return r.db.WithTx(staffOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE staff_outbox SET last_error=left($2,1000) WHERE id=$1`, id, message)
		return err
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

// CreateAssignment atomically stores teacher scope and its staff.assigned event.
func (r *Repository) CreateAssignment(ctx context.Context, tenantID string, assignment *domain.Assignment, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("staff: encode assignment event: %w", err)
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO staff_assignments (id, tenant_id, staff_id, class_id, subject_id, role, assigned_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, assignment.ID, tenantID, assignment.StaffID, assignment.ClassID, assignment.SubjectID, assignment.Role, assignment.AssignedAt); err != nil {
			return fmt.Errorf("staff: create assignment: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO staff_outbox (id,tenant_id,event_type,payload) VALUES ($1,$2,'staff.assigned.v1',$3)`, uuid.NewString(), tenantID, encoded); err != nil {
			return fmt.Errorf("staff: enqueue assignment event: %w", err)
		}
		return nil
	})
}

func (r *Repository) ListAssignments(ctx context.Context, tenantID, staffID string, limit int, cursor string) ([]*domain.Assignment, string, error) {
	items := make([]*domain.Assignment, 0, limit)
	var next string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		query := `SELECT id,tenant_id,staff_id,class_id,subject_id,role,assigned_at FROM staff_assignments WHERE tenant_id=$1 AND staff_id=$2`
		args := []any{tenantID, staffID}
		if cursor != "" {
			query += ` AND (assigned_at,id) > (SELECT assigned_at,id FROM staff_assignments WHERE tenant_id=$1 AND staff_id=$2 AND id=$3)`
			args = append(args, cursor)
		}
		query += fmt.Sprintf(" ORDER BY assigned_at,id LIMIT $%d", len(args)+1)
		args = append(args, limit)
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("staff: list assignments: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item domain.Assignment
			if err := rows.Scan(&item.ID, &item.TenantID, &item.StaffID, &item.ClassID, &item.SubjectID, &item.Role, &item.AssignedAt); err != nil {
				return err
			}
			items = append(items, &item)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(items) == limit && len(items) > 0 {
			next = items[len(items)-1].ID
		}
		return nil
	})
	return items, next, err
}

func (r *Repository) DeleteAssignment(ctx context.Context, tenantID, staffID, assignmentID string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM staff_assignments WHERE id=$1 AND staff_id=$2 AND tenant_id=$3`, assignmentID, staffID, tenantID)
		if err != nil {
			return fmt.Errorf("staff: delete assignment: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *Repository) ListAssignmentClassIDs(ctx context.Context, tenantID, staffID string) ([]string, error) {
	ids := make([]string, 0)
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT DISTINCT class_id::text FROM staff_assignments WHERE tenant_id=$1 AND staff_id=$2 ORDER BY class_id::text`, tenantID, staffID)
		if err != nil {
			return fmt.Errorf("staff: list assignment class ids: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return ids, err
}

func (r *Repository) ListAssignmentSubjectIDs(ctx context.Context, tenantID, staffID string) ([]string, error) {
	ids := make([]string, 0)
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT DISTINCT subject_id::text FROM staff_assignments WHERE tenant_id=$1 AND staff_id=$2 AND subject_id IS NOT NULL ORDER BY subject_id::text`, tenantID, staffID)
		if err != nil {
			return fmt.Errorf("staff: list assignment subject ids: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return ids, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanStaff(row scanner) (*domain.Staff, error) {
	var s domain.Staff
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.FirstName, &s.LastName, &s.StaffType,
		&s.Email, &s.UserID, &s.StaffCode, &s.Status, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}
