package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auraedu/academic-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CommitAcademicLifecycle(
	ctx context.Context,
	tenantID string,
	m ports.AcademicMutation,
	eventType string,
	payload map[string]any,
) error {
	return commitAcademicLifecycle(ctx, r.db, tenantID, m, eventType, payload)
}
func (r *TermRepository) CommitAcademicLifecycle(
	ctx context.Context,
	tenantID string,
	m ports.AcademicMutation,
	eventType string,
	payload map[string]any,
) error {
	return commitAcademicLifecycle(ctx, r.db, tenantID, m, eventType, payload)
}
func (r *ClassRepository) CommitAcademicLifecycle(
	ctx context.Context,
	tenantID string,
	m ports.AcademicMutation,
	eventType string,
	payload map[string]any,
) error {
	return commitAcademicLifecycle(ctx, r.db, tenantID, m, eventType, payload)
}
func (r *SubjectRepository) CommitAcademicLifecycle(
	ctx context.Context,
	tenantID string,
	m ports.AcademicMutation,
	eventType string,
	payload map[string]any,
) error {
	return commitAcademicLifecycle(ctx, r.db, tenantID, m, eventType, payload)
}

func commitAcademicLifecycle(
	ctx context.Context,
	database *db.DB,
	tenantID string,
	m ports.AcademicMutation,
	eventType string,
	payload map[string]any,
) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return database.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		switch m.Kind {
		case ports.AcademicMutationYearCreate:
			y := m.Year
			_, err = tx.Exec(ctx, `
				INSERT INTO academic_years (
					id, tenant_id, name, code, start_date, end_date, status,
					is_current, created_at, updated_at
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
				y.ID, tenantID, y.Name, y.Code, y.StartDate, y.EndDate,
				y.Status, y.IsCurrent, y.CreatedAt, y.UpdatedAt,
			)
		case ports.AcademicMutationYearUpdate:
			y := m.Year
			_, err = tx.Exec(ctx, `
				UPDATE academic_years
				SET name=$3, code=$4, start_date=$5, end_date=$6, status=$7,
					is_current=$8, updated_at=$9
				WHERE id=$1 AND tenant_id=$2`,
				y.ID, tenantID, y.Name, y.Code, y.StartDate, y.EndDate,
				y.Status, y.IsCurrent, y.UpdatedAt,
			)
		case ports.AcademicMutationYearDelete:
			_, err = tx.Exec(ctx, `DELETE FROM academic_years WHERE id=$1 AND tenant_id=$2`, m.Year.ID, tenantID)
		case ports.AcademicMutationTermUpdate:
			t := m.Term
			_, err = tx.Exec(ctx, `
				UPDATE terms SET name=$3, start_date=$4, end_date=$5, updated_at=$6
				WHERE id=$1 AND tenant_id=$2`,
				t.ID, tenantID, t.Name, t.StartDate, t.EndDate, t.UpdatedAt,
			)
		case ports.AcademicMutationTermDelete:
			_, err = tx.Exec(ctx, `DELETE FROM terms WHERE id=$1 AND tenant_id=$2`, m.Term.ID, tenantID)
		case ports.AcademicMutationClassCreate:
			c := m.Class
			_, err = tx.Exec(ctx, `
				INSERT INTO classes (
					id, tenant_id, name, academic_year_id, class_teacher_id,
					capacity, created_at, updated_at
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
				c.ID, tenantID, c.Name, c.AcademicYearID, c.ClassTeacherID,
				c.Capacity, c.CreatedAt, c.UpdatedAt,
			)
		case ports.AcademicMutationClassUpdate:
			c := m.Class
			_, err = tx.Exec(ctx, `
				UPDATE classes
				SET name=$3, class_teacher_id=$4, capacity=$5, updated_at=$6
				WHERE id=$1 AND tenant_id=$2`,
				c.ID, tenantID, c.Name, c.ClassTeacherID, c.Capacity, c.UpdatedAt,
			)
		case ports.AcademicMutationClassDelete:
			_, err = tx.Exec(ctx, `DELETE FROM classes WHERE id=$1 AND tenant_id=$2`, m.Class.ID, tenantID)
		case ports.AcademicMutationSubjectCreate:
			s := m.Subject
			_, err = tx.Exec(ctx, `
				INSERT INTO subjects (
					id, tenant_id, name, code, description, created_at, updated_at
				) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				s.ID, tenantID, s.Name, s.Code, s.Description, s.CreatedAt, s.UpdatedAt,
			)
		case ports.AcademicMutationSubjectUpdate:
			s := m.Subject
			_, err = tx.Exec(ctx, `
				UPDATE subjects
				SET name=$3, code=$4, description=$5, updated_at=$6
				WHERE id=$1 AND tenant_id=$2`,
				s.ID, tenantID, s.Name, s.Code, s.Description, s.UpdatedAt,
			)
		case ports.AcademicMutationSubjectDelete:
			_, err = tx.Exec(ctx, `DELETE FROM subjects WHERE id=$1 AND tenant_id=$2`, m.Subject.ID, tenantID)
		default:
			return fmt.Errorf("academic: unsupported lifecycle mutation %q", m.Kind)
		}
		if err != nil {
			return fmt.Errorf("academic: lifecycle mutation: %w", err)
		}
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO academic_outbox(id,tenant_id,event_type,payload) VALUES($1,$2,$3,$4)`,
			uuid.NewString(), tenantID, eventType, encoded,
		); err != nil {
			return fmt.Errorf("academic: enqueue lifecycle event: %w", err)
		}
		return nil
	})
}

func academicOutboxContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__academic_outbox__"})
}
func (r *Repository) ClaimPendingAcademicEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := make([]ports.OutboxEvent, 0, limit)
	err := r.db.WithTx(academicOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE academic_outbox
			SET attempts=attempts+1,
				next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second')
			WHERE id IN (
				SELECT id FROM academic_outbox
				WHERE published_at IS NULL AND next_attempt_at<=now()
				ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1
			)
			RETURNING id::text,tenant_id,event_type,payload`, limit)
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
func (r *Repository) MarkAcademicEventPublished(ctx context.Context, id string) error {
	return r.db.WithTx(academicOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE academic_outbox SET published_at=now(),last_error=NULL WHERE id=$1`, id)
		return err
	})
}
func (r *Repository) MarkAcademicEventFailed(ctx context.Context, id, message string) error {
	return r.db.WithTx(academicOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE academic_outbox SET last_error=left($2,1000) WHERE id=$1`, id, message)
		return err
	})
}
