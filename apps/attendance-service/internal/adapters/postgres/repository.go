// Package postgres provides the Postgres implementation of the attendance repository port.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
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
	_ ports.Repository          = (*Repository)(nil)
	_ ports.LifecycleRepository = (*Repository)(nil)
	_ ports.OutboxRepository    = (*Repository)(nil)
)

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

// UpsertMany inserts records in a single transaction, updating the mutable fields when a
// live row already exists for (tenant_id, student_id, academic_year_id, date) — the
// uniqueness rule enforced by idx_attendance_records_unique_attendance in
// migrations/0001_init.sql. Bulk marks are therefore idempotent: retrying the same
// request converges to the same rows. Each record is rewritten in place with the
// persisted row (on conflict the pre-existing id and created_at are kept).
func (r *Repository) UpsertMany(ctx context.Context, tenantID string, records []*domain.AttendanceRecord) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		for _, rec := range records {
			row := tx.QueryRow(ctx, `
				INSERT INTO attendance_records (id, tenant_id, student_id, academic_year_id, class_id, subject_id, date, status, reason, marked_by, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				ON CONFLICT (tenant_id, student_id, academic_year_id, date) WHERE deleted_at IS NULL
				DO UPDATE SET status = EXCLUDED.status, reason = EXCLUDED.reason, marked_by = EXCLUDED.marked_by,
					class_id = EXCLUDED.class_id, subject_id = EXCLUDED.subject_id, updated_at = EXCLUDED.updated_at
				RETURNING id, tenant_id, student_id, academic_year_id, date, status, reason, marked_by, class_id, subject_id, created_at, updated_at
			`, rec.ID, tenantID, rec.StudentID, rec.AcademicYearID, rec.ClassID, rec.SubjectID,
				rec.Date, rec.Status, rec.Reason, rec.MarkedBy, rec.CreatedAt, rec.UpdatedAt)
			got, err := scanRecord(row)
			if err != nil {
				return fmt.Errorf("attendance: upsert record for student %s: %w", rec.StudentID, err)
			}
			*rec = *got
		}
		return nil
	})
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.AttendanceRecord, error) {
	var rec *domain.AttendanceRecord
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, student_id, academic_year_id, date, status, reason, marked_by, class_id, subject_id, created_at, updated_at
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
	} else if filter.StudentIDs != nil {
		args = append(args, filter.StudentIDs)
		where += fmt.Sprintf(" AND student_id = ANY($%d)", len(args))
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
		SELECT id, tenant_id, student_id, academic_year_id, date, status, reason, marked_by, class_id, subject_id, created_at, updated_at
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

func (r *Repository) CommitAttendanceLifecycle(
	ctx context.Context,
	tenantID, mutation string,
	records []*domain.AttendanceRecord,
	eventType string,
	metas []map[string]any,
) error {
	if len(records) == 0 {
		return errors.New("attendance: lifecycle mutation requires at least one record")
	}
	if eventType == "" {
		return errors.New("attendance: lifecycle event type is required")
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := applyAttendanceMutation(ctx, tx, tenantID, mutation, records); err != nil {
			return err
		}
		return enqueueAttendanceEvents(ctx, tx, tenantID, eventType, records, metas)
	})
}

func applyAttendanceMutation(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, mutation string,
	records []*domain.AttendanceRecord,
) error {
	switch mutation {
	case ports.AttendanceMutationCreate:
		rec := records[0]
		if _, err := tx.Exec(ctx, `
				INSERT INTO attendance_records(
					id,tenant_id,student_id,academic_year_id,date,status,
					reason,marked_by,created_at,updated_at
				) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			rec.ID, tenantID, rec.StudentID, rec.AcademicYearID, rec.Date,
			rec.Status, rec.Reason, rec.MarkedBy, rec.CreatedAt, rec.UpdatedAt,
		); err != nil {
			return err
		}
	case ports.AttendanceMutationBulkUpsert:
		for _, rec := range records {
			row := tx.QueryRow(ctx, `
					INSERT INTO attendance_records(
						id,tenant_id,student_id,academic_year_id,class_id,subject_id,
						date,status,reason,marked_by,created_at,updated_at
					) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
					ON CONFLICT(tenant_id,student_id,academic_year_id,date)
					WHERE deleted_at IS NULL DO UPDATE SET
						status=EXCLUDED.status,reason=EXCLUDED.reason,
						marked_by=EXCLUDED.marked_by,class_id=EXCLUDED.class_id,
						subject_id=EXCLUDED.subject_id,updated_at=EXCLUDED.updated_at
					RETURNING id,tenant_id,student_id,academic_year_id,date,status,
						reason,marked_by,class_id,subject_id,created_at,updated_at`,
				rec.ID, tenantID, rec.StudentID, rec.AcademicYearID, rec.ClassID,
				rec.SubjectID, rec.Date, rec.Status, rec.Reason, rec.MarkedBy,
				rec.CreatedAt, rec.UpdatedAt)
			got, err := scanRecord(row)
			if err != nil {
				return err
			}
			*rec = *got
		}
	case ports.AttendanceMutationUpdate:
		rec := records[0]
		result, err := tx.Exec(ctx, `
				UPDATE attendance_records SET student_id=$3,academic_year_id=$4,
					date=$5,status=$6,reason=$7,marked_by=$8,updated_at=$9
				WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`,
			rec.ID, tenantID, rec.StudentID, rec.AcademicYearID, rec.Date,
			rec.Status, rec.Reason, rec.MarkedBy, rec.UpdatedAt)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
	case ports.AttendanceMutationDelete:
		result, err := tx.Exec(ctx, `UPDATE attendance_records SET deleted_at=now() WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, records[0].ID, tenantID)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
	default:
		return fmt.Errorf("attendance: unsupported lifecycle mutation %q", mutation)
	}
	return nil
}

func enqueueAttendanceEvents(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, eventType string,
	records []*domain.AttendanceRecord,
	metas []map[string]any,
) error {
	for i, rec := range records {
		var meta map[string]any
		if i < len(metas) {
			meta = metas[i]
		}
		payload, err := json.Marshal(ports.AttendanceEventData(rec, meta))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
				INSERT INTO attendance_outbox(id,tenant_id,event_type,payload)
				VALUES($1,$2,$3,$4)`, uuid.NewString(), tenantID, eventType, payload); err != nil {
			return fmt.Errorf("attendance: enqueue lifecycle event: %w", err)
		}
	}
	return nil
}

func attendanceOutboxContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__attendance_outbox__"})
}

func (r *Repository) ClaimPendingAttendanceEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := make([]ports.OutboxEvent, 0, limit)
	err := r.db.WithTx(attendanceOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE attendance_outbox SET attempts=attempts+1,
				next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second')
			WHERE id IN(
				SELECT id FROM attendance_outbox
				WHERE published_at IS NULL AND next_attempt_at<=now()
				ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1
			) RETURNING id::text,tenant_id,event_type,payload`, limit)
		if err != nil {
			return err
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

func (r *Repository) MarkAttendanceEventPublished(ctx context.Context, id string) error {
	return r.db.WithTx(attendanceOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE attendance_outbox SET published_at=now(),last_error=NULL WHERE id=$1`, id)
		return err
	})
}

func (r *Repository) MarkAttendanceEventFailed(ctx context.Context, id, msg string) error {
	return r.db.WithTx(attendanceOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE attendance_outbox SET last_error=left($2,1000) WHERE id=$1`, id, msg)
		return err
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
		&rec.Status, &rec.Reason, &rec.MarkedBy, &rec.ClassID, &rec.SubjectID, &rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return nil, err
	}
	rec.Date = domain.Date{Time: date}
	return &rec, nil
}
