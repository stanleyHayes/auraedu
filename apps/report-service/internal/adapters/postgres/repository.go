// Package postgres persists report aggregates in PostgreSQL.
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
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
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

var _ ports.Repository = (*Repository)(nil)
var _ ports.OutboxRepository = (*Repository)(nil)
var _ ports.LifecycleRepository = (*Repository)(nil)

// NewRepository creates a Postgres-backed report repository.
func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

// CommitReportTemplateLifecycle persists a template transition and its event
// in one transaction. If the outbox insert fails, the aggregate mutation is
// rolled back so consumers can never silently miss a committed transition.
func (r *Repository) CommitReportTemplateLifecycle(ctx context.Context, tenantID string, template *domain.ReportTemplate, mutation, eventType string, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("report: encode template lifecycle event: %w", err)
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		switch mutation {
		case ports.ReportMutationCreate:
			_, err = tx.Exec(ctx, `
				INSERT INTO report_templates (id, tenant_id, name, academic_year_id, body_template, status, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`, template.ID, tenantID, template.Name, template.AcademicYearID, template.BodyTemplate, template.Status, template.CreatedAt, template.UpdatedAt)
		case ports.ReportMutationUpdate:
			var tag pgconn.CommandTag
			tag, err = tx.Exec(ctx, `
				UPDATE report_templates
				SET name=$3, academic_year_id=$4, body_template=$5, status=$6, updated_at=$7
				WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL
			`, template.ID, tenantID, template.Name, template.AcademicYearID, template.BodyTemplate, template.Status, template.UpdatedAt)
			if err == nil && tag.RowsAffected() != 1 {
				return domain.ErrNotFound
			}
		case ports.ReportMutationDelete:
			var tag pgconn.CommandTag
			tag, err = tx.Exec(ctx, `
				UPDATE report_templates SET deleted_at=$3
				WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL
			`, template.ID, tenantID, time.Now().UTC())
			if err == nil && tag.RowsAffected() != 1 {
				return domain.ErrNotFound
			}
		default:
			return fmt.Errorf("report: unsupported template lifecycle mutation %q", mutation)
		}
		if err != nil {
			return fmt.Errorf("report: commit template lifecycle: %w", err)
		}
		return enqueueReportEvent(ctx, tx, tenantID, eventType, encoded)
	})
}

// CommitReportCardLifecycle persists a report-card transition and its event in
// one transaction, closing the database-to-NATS event-loss window.
func (r *Repository) CommitReportCardLifecycle(ctx context.Context, tenantID string, card *domain.ReportCard, mutation, eventType string, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("report: encode card lifecycle event: %w", err)
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		switch mutation {
		case ports.ReportMutationCreate:
			_, err = tx.Exec(ctx, `
				INSERT INTO report_cards (id, tenant_id, student_id, academic_year_id, term_id, template_id, status, pdf_path, generated_at, created_at, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			`, card.ID, tenantID, card.StudentID, nullIfEmpty(card.AcademicYearID), nullIfEmpty(card.TermID), nullIfEmpty(card.TemplateID), card.Status, card.PDFPath, card.GeneratedAt, card.CreatedAt, card.UpdatedAt)
			if isUniqueViolation(err) {
				return fmt.Errorf("report: create report card: %w", domain.ErrConflict)
			}
		case ports.ReportMutationUpdate:
			var tag pgconn.CommandTag
			tag, err = tx.Exec(ctx, `
				UPDATE report_cards
				SET student_id=$3, academic_year_id=$4, term_id=$5, template_id=$6, status=$7, pdf_path=$8, generated_at=$9, updated_at=$10
				WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL
			`, card.ID, tenantID, card.StudentID, nullIfEmpty(card.AcademicYearID), nullIfEmpty(card.TermID), nullIfEmpty(card.TemplateID), card.Status, card.PDFPath, card.GeneratedAt, card.UpdatedAt)
			if err == nil && tag.RowsAffected() != 1 {
				return domain.ErrNotFound
			}
		case ports.ReportMutationDelete:
			var tag pgconn.CommandTag
			tag, err = tx.Exec(ctx, `
				UPDATE report_cards SET deleted_at=$3
				WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL
			`, card.ID, tenantID, time.Now().UTC())
			if err == nil && tag.RowsAffected() != 1 {
				return domain.ErrNotFound
			}
		default:
			return fmt.Errorf("report: unsupported card lifecycle mutation %q", mutation)
		}
		if err != nil {
			return fmt.Errorf("report: commit card lifecycle: %w", err)
		}
		return enqueueReportEvent(ctx, tx, tenantID, eventType, encoded)
	})
}

func enqueueReportEvent(ctx context.Context, tx pgx.Tx, tenantID, eventType string, payload []byte) error {
	if eventType == "" {
		return errors.New("report: lifecycle event type is required")
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO report_outbox (id, tenant_id, event_type, payload, created_at, next_attempt_at)
		VALUES ($1, $2, $3, $4, now(), now())
	`, uuid.NewString(), tenantID, eventType, payload)
	if err != nil {
		return fmt.Errorf("report: enqueue lifecycle event: %w", err)
	}
	return nil
}

// --- Report templates. ---

func (r *Repository) CreateReportTemplate(ctx context.Context, tenantID string, t *domain.ReportTemplate) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO report_templates (id, tenant_id, name, academic_year_id, body_template, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, t.ID, tenantID, t.Name, t.AcademicYearID, t.BodyTemplate, t.Status, t.CreatedAt, t.UpdatedAt)
		if err != nil {
			return fmt.Errorf("report: create report template: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetReportTemplateByID(ctx context.Context, tenantID, id string) (*domain.ReportTemplate, error) {
	var t *domain.ReportTemplate
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, name, academic_year_id, body_template, status, created_at, updated_at
			FROM report_templates
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, id, tenantID)
		got, err := scanReportTemplate(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("report: get report template: %w", err)
		}
		t = got
		return nil
	})
	return t, err
}

func (r *Repository) ListReportTemplates(
	ctx context.Context,
	tenantID string,
	filter ports.ReportTemplateListFilter,
) ([]*domain.ReportTemplate, string, error) {
	var out []*domain.ReportTemplate
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listReportTemplatesQuery(ctx, tx, tenantID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			rec, err := scanReportTemplate(rows)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("report: list report templates rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listReportTemplatesQuery(ctx context.Context, tx pgx.Tx, tenantID string, filter ports.ReportTemplateListFilter) (pgx.Rows, error) {
	args := []any{tenantID}
	where := "tenant_id = $1 AND deleted_at IS NULL"

	if filter.AcademicYearID != "" {
		args = append(args, filter.AcademicYearID)
		where += fmt.Sprintf(" AND academic_year_id = $%d", len(args))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM report_templates WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, name, academic_year_id, body_template, status, created_at, updated_at
		FROM report_templates
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *Repository) UpdateReportTemplate(ctx context.Context, tenantID string, t *domain.ReportTemplate) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE report_templates
			SET name = $3, academic_year_id = $4, body_template = $5, status = $6, updated_at = $7
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, t.ID, tenantID, t.Name, t.AcademicYearID, t.BodyTemplate, t.Status, t.UpdatedAt)
		if err != nil {
			return fmt.Errorf("report: update report template: %w", err)
		}
		return nil
	})
}

func (r *Repository) DeleteReportTemplate(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		now := time.Now().UTC()
		_, err := tx.Exec(ctx, `
			UPDATE report_templates
			SET deleted_at = $3
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, id, tenantID, now)
		if err != nil {
			return fmt.Errorf("report: delete report template: %w", err)
		}
		return nil
	})
}

// --- Report cards. ---

func (r *Repository) CreateReportCard(ctx context.Context, tenantID string, c *domain.ReportCard) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO report_cards (id, tenant_id, student_id, academic_year_id, term_id, template_id, status, pdf_path, generated_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, c.ID, tenantID, c.StudentID, nullIfEmpty(c.AcademicYearID), nullIfEmpty(c.TermID), nullIfEmpty(c.TemplateID), c.Status, c.PDFPath, c.GeneratedAt, c.CreatedAt, c.UpdatedAt)
		if isUniqueViolation(err) {
			return fmt.Errorf("report: create report card: %w", domain.ErrConflict)
		}
		if err != nil {
			return fmt.Errorf("report: create report card: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetReportCardByID(ctx context.Context, tenantID, id string) (*domain.ReportCard, error) {
	var c *domain.ReportCard
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, student_id, COALESCE(academic_year_id::text, ''), COALESCE(term_id::text, ''), COALESCE(template_id::text, ''), status, pdf_path, generated_at, created_at, updated_at
			FROM report_cards
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, id, tenantID)
		got, err := scanReportCard(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("report: get report card: %w", err)
		}
		c = got
		return nil
	})
	return c, err
}

func (r *Repository) ListReportCards(ctx context.Context, tenantID string, filter ports.ReportCardListFilter) ([]*domain.ReportCard, string, error) {
	var out []*domain.ReportCard
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listReportCardsQuery(ctx, tx, tenantID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			rec, err := scanReportCard(rows)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("report: list report cards rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listReportCardsQuery(ctx context.Context, tx pgx.Tx, tenantID string, filter ports.ReportCardListFilter) (pgx.Rows, error) {
	args := []any{tenantID}
	where := "tenant_id = $1 AND deleted_at IS NULL"

	if filter.AcademicYearID != "" {
		args = append(args, filter.AcademicYearID)
		where += fmt.Sprintf(" AND academic_year_id = $%d", len(args))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.StudentID != "" {
		args = append(args, filter.StudentID)
		where += fmt.Sprintf(" AND student_id = $%d", len(args))
	}
	if filter.StudentIDs != nil {
		if len(filter.StudentIDs) == 0 {
			where += " AND false"
		} else {
			args = append(args, filter.StudentIDs)
			where += fmt.Sprintf(" AND student_id = ANY($%d)", len(args))
		}
	}
	if filter.TemplateID != "" {
		args = append(args, filter.TemplateID)
		where += fmt.Sprintf(" AND template_id = $%d", len(args))
	}

	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM report_cards WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, student_id, COALESCE(academic_year_id::text, ''), COALESCE(term_id::text, ''), COALESCE(template_id::text, ''), status, pdf_path, generated_at, created_at, updated_at
		FROM report_cards
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *Repository) UpdateReportCard(ctx context.Context, tenantID string, c *domain.ReportCard) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE report_cards
			SET student_id = $3, academic_year_id = $4, term_id = $5, template_id = $6, status = $7, pdf_path = $8, generated_at = $9, updated_at = $10
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, c.ID, tenantID, c.StudentID, nullIfEmpty(c.AcademicYearID), nullIfEmpty(c.TermID), nullIfEmpty(c.TemplateID), c.Status, c.PDFPath, c.GeneratedAt, c.UpdatedAt)
		if err != nil {
			return fmt.Errorf("report: update report card: %w", err)
		}
		return nil
	})
}

func (r *Repository) DeleteReportCard(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		now := time.Now().UTC()
		_, err := tx.Exec(ctx, `
			UPDATE report_cards
			SET deleted_at = $3
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, id, tenantID, now)
		if err != nil {
			return fmt.Errorf("report: delete report card: %w", err)
		}
		return nil
	})
}

// ListTranscriptReportCards returns every published academic period for one
// learner. Archived cards remain part of the permanent academic record.
func (r *Repository) ListTranscriptReportCards(ctx context.Context, tenantID, studentID string) ([]*domain.ReportCard, error) {
	var out []*domain.ReportCard
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, tenant_id, student_id, COALESCE(academic_year_id::text, ''), COALESCE(term_id::text, ''), COALESCE(template_id::text, ''), status, pdf_path, generated_at, created_at, updated_at
			FROM report_cards
			WHERE tenant_id = $1 AND student_id = $2
			  AND status IN ('published', 'archived') AND deleted_at IS NULL
			ORDER BY generated_at ASC NULLS LAST, created_at ASC, id ASC
		`, tenantID, studentID)
		if err != nil {
			return fmt.Errorf("report: list transcript report cards: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			card, err := scanReportCard(rows)
			if err != nil {
				return fmt.Errorf("report: scan transcript report card: %w", err)
			}
			out = append(out, card)
		}
		return rows.Err()
	})
	return out, err
}

// FindDraftReportCard returns the DRAFT report card for a student and period.
// With a non-empty termID, cards with a NULL term (period not yet assigned)
// also match and an exact term match is preferred. With an empty termID (e.g.
// attendance events, which carry no term) any draft matches and the most
// recently created wins — the current period's card.
func (r *Repository) FindDraftReportCard(ctx context.Context, tenantID, studentID, termID string) (*domain.ReportCard, error) {
	var c *domain.ReportCard
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// $3 = '' means "no term given": match every draft for the student, most
		// recent first. Otherwise exact term matches win over NULL-term drafts.
		// The sort key never evaluates to NULL: "false AND NULL" stays false.
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, student_id, COALESCE(academic_year_id::text, ''), COALESCE(term_id::text, ''), COALESCE(template_id::text, ''), status, pdf_path, generated_at, created_at, updated_at
			FROM report_cards
			WHERE tenant_id = $1 AND student_id = $2 AND status = 'draft' AND deleted_at IS NULL
			  AND ($3 = '' OR term_id IS NULL OR term_id::text = $3)
			ORDER BY (term_id IS NOT NULL AND term_id::text = $3) DESC, created_at DESC, id DESC
			LIMIT 1
		`, tenantID, studentID, termID)
		got, err := scanReportCard(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("report: find draft report card: %w", err)
		}
		c = got
		return nil
	})
	return c, err
}

// --- Materialized entries. ---

// UpsertScoreEntry inserts or rewrites the score entry keyed by
// (report_card_id, source_key): replayed events and corrected scores converge
// to the same row.
func (r *Repository) UpsertScoreEntry(ctx context.Context, tenantID string, e *domain.ScoreEntry) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO report_card_score_entries (id, tenant_id, report_card_id, student_id, subject_id, source_key, score, max_score, last_event_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (report_card_id, source_key) DO UPDATE
			SET subject_id = EXCLUDED.subject_id,
			    score = EXCLUDED.score,
			    max_score = EXCLUDED.max_score,
			    last_event_id = EXCLUDED.last_event_id,
			    updated_at = EXCLUDED.updated_at
		`, e.ID, tenantID, e.ReportCardID, e.StudentID, nullIfEmpty(e.SubjectID), e.SourceKey, e.Score, e.MaxScore, e.LastEventID, e.CreatedAt, e.UpdatedAt)
		if err != nil {
			return fmt.Errorf("report: upsert score entry: %w", err)
		}
		return nil
	})
}

// UpsertAttendanceEntry inserts or rewrites the attendance entry keyed by
// (report_card_id, entry_date): re-marks update the day in place.
func (r *Repository) UpsertAttendanceEntry(ctx context.Context, tenantID string, e *domain.AttendanceEntry) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO report_card_attendance_entries (id, tenant_id, report_card_id, student_id, entry_date, status, last_event_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (report_card_id, entry_date) DO UPDATE
			SET status = EXCLUDED.status,
			    last_event_id = EXCLUDED.last_event_id,
			    updated_at = EXCLUDED.updated_at
		`, e.ID, tenantID, e.ReportCardID, e.StudentID, e.Date, e.Status, e.LastEventID, e.CreatedAt, e.UpdatedAt)
		if err != nil {
			return fmt.Errorf("report: upsert attendance entry: %w", err)
		}
		return nil
	})
}

// ListScoreEntries returns all materialized score entries for a report card.
func (r *Repository) ListScoreEntries(ctx context.Context, tenantID, reportCardID string) ([]*domain.ScoreEntry, error) {
	var out []*domain.ScoreEntry
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, tenant_id, report_card_id, student_id, COALESCE(subject_id::text, ''), source_key, score, max_score, last_event_id, created_at, updated_at
			FROM report_card_score_entries
			WHERE tenant_id = $1 AND report_card_id = $2
			ORDER BY created_at ASC, id ASC
		`, tenantID, reportCardID)
		if err != nil {
			return fmt.Errorf("report: list score entries: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var e domain.ScoreEntry
			if err := rows.Scan(
				&e.ID, &e.TenantID, &e.ReportCardID, &e.StudentID, &e.SubjectID, &e.SourceKey,
				&e.Score, &e.MaxScore, &e.LastEventID, &e.CreatedAt, &e.UpdatedAt,
			); err != nil {
				return fmt.Errorf("report: scan score entry: %w", err)
			}
			out = append(out, &e)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("report: list score entries rows: %w", err)
		}
		return nil
	})
	return out, err
}

// ListAttendanceEntries returns all materialized attendance entries for a report card.
func (r *Repository) ListAttendanceEntries(ctx context.Context, tenantID, reportCardID string) ([]*domain.AttendanceEntry, error) {
	var out []*domain.AttendanceEntry
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, tenant_id, report_card_id, student_id, entry_date, status, last_event_id, created_at, updated_at
			FROM report_card_attendance_entries
			WHERE tenant_id = $1 AND report_card_id = $2
			ORDER BY entry_date ASC, id ASC
		`, tenantID, reportCardID)
		if err != nil {
			return fmt.Errorf("report: list attendance entries: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var e domain.AttendanceEntry
			if err := rows.Scan(
				&e.ID, &e.TenantID, &e.ReportCardID, &e.StudentID, &e.Date, &e.Status, &e.LastEventID, &e.CreatedAt, &e.UpdatedAt,
			); err != nil {
				return fmt.Errorf("report: scan attendance entry: %w", err)
			}
			out = append(out, &e)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("report: list attendance entries rows: %w", err)
		}
		return nil
	})
	return out, err
}

// EnqueueReportGeneration atomically changes the aggregate state and writes a
// replay-safe durable job. A card already generating cannot be queued twice.
func (r *Repository) EnqueueReportGeneration(ctx context.Context, tenantID, reportCardID string) (*domain.ReportCard, error) {
	var card *domain.ReportCard
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			UPDATE report_cards
			SET status = 'generating', updated_at = now()
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
			  AND status IN ('draft', 'published')
			RETURNING id, tenant_id, student_id, COALESCE(academic_year_id::text, ''),
			          COALESCE(term_id::text, ''), COALESCE(template_id::text, ''),
			          status, pdf_path, generated_at, created_at, updated_at
		`, reportCardID, tenantID)
		got, err := scanReportCard(row)
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if checkErr := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM report_cards WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL)`, reportCardID, tenantID).Scan(&exists); checkErr != nil {
				return checkErr
			}
			if !exists {
				return domain.ErrNotFound
			}
			return fmt.Errorf("%w: report card cannot be generated from its current status", domain.ErrConflict)
		}
		if err != nil {
			return fmt.Errorf("report: queue report card: %w", err)
		}
		card = got
		_, err = tx.Exec(ctx, `
			INSERT INTO report_generation_jobs (report_card_id, tenant_id, status, attempts, next_attempt_at, created_at, updated_at)
			VALUES ($1, $2, 'queued', 0, now(), now(), now())
			ON CONFLICT (report_card_id) DO UPDATE
			SET tenant_id=EXCLUDED.tenant_id, status='queued', attempts=0,
			    next_attempt_at=now(), lease_expires_at=NULL, last_error=NULL,
			    completed_at=NULL, updated_at=now()
		`, reportCardID, tenantID)
		if err != nil {
			return fmt.Errorf("report: enqueue generation job: %w", err)
		}
		return nil
	})
	return card, err
}

func reportPlatformContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__report_generation__"})
}

// ClaimReportGeneration leases one ready or abandoned job without blocking
// other workers. ErrNotFound means the queue is currently empty.
func (r *Repository) ClaimReportGeneration(ctx context.Context, lease time.Duration) (*domain.GenerationJob, error) {
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	var job domain.GenerationJob
	err := r.db.WithTx(reportPlatformContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			WITH candidate AS (
				SELECT report_card_id
				FROM report_generation_jobs
				WHERE (status = 'queued' AND next_attempt_at <= now())
				   OR (status = 'running' AND lease_expires_at < now())
				ORDER BY next_attempt_at, created_at
				FOR UPDATE SKIP LOCKED
				LIMIT 1
			)
			UPDATE report_generation_jobs AS job
			SET status='running', attempts=job.attempts+1,
			    lease_expires_at=now()+make_interval(secs => $1), updated_at=now()
			FROM candidate
			WHERE job.report_card_id=candidate.report_card_id
			RETURNING job.report_card_id, job.tenant_id, job.attempts,
			          job.lease_expires_at, job.next_attempt_at
		`, int(lease.Seconds()))
		if err := row.Scan(&job.ReportCardID, &job.TenantID, &job.Attempts, &job.LeaseExpires, &job.NextAttemptAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("report: claim generation job: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// CompleteReportGeneration commits the published storage key and terminal job
// state together, preventing a job from completing without its aggregate.
func (r *Repository) CompleteReportGeneration(ctx context.Context, job *domain.GenerationJob, storagePath string) (*domain.ReportCard, error) {
	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: job.TenantID})
	var card *domain.ReportCard
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			UPDATE report_cards
			SET status='published', pdf_path=$3, generated_at=now(), updated_at=now()
			WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL AND status='generating'
			RETURNING id, tenant_id, student_id, COALESCE(academic_year_id::text, ''),
			          COALESCE(term_id::text, ''), COALESCE(template_id::text, ''),
			          status, pdf_path, generated_at, created_at, updated_at
		`, job.ReportCardID, job.TenantID, storagePath)
		got, err := scanReportCard(row)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrConflict
		}
		if err != nil {
			return fmt.Errorf("report: publish generated card: %w", err)
		}
		card = got
		tag, err := tx.Exec(ctx, `
			UPDATE report_generation_jobs
			SET status='completed', completed_at=now(), lease_expires_at=NULL,
			    last_error=NULL, updated_at=now()
			WHERE report_card_id=$1 AND tenant_id=$2 AND status='running'
		`, job.ReportCardID, job.TenantID)
		if err != nil {
			return fmt.Errorf("report: complete generation job: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrConflict
		}
		payload, err := json.Marshal(map[string]any{
			"report_card_id":   card.ID,
			"student_id":       card.StudentID,
			"academic_year_id": card.AcademicYearID,
			"term_id":          card.TermID,
			"template_id":      card.TemplateID,
			"status":           card.Status,
			"file_url":         "/api/v1/report-cards/" + card.ID + "/download",
			"generated_at":     card.GeneratedAt.Format(time.RFC3339),
		})
		if err != nil {
			return fmt.Errorf("report: encode published event: %w", err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO report_outbox (id, tenant_id, event_type, payload, created_at, next_attempt_at)
			VALUES ($1, $2, 'report.published.v1', $3, now(), now())
		`, uuid.NewString(), job.TenantID, payload)
		if err != nil {
			return fmt.Errorf("report: enqueue published event: %w", err)
		}
		return nil
	})
	return card, err
}

// ClaimPendingReportEvents reserves a bounded batch by moving its retry clock.
// Concurrent workers use SKIP LOCKED and therefore never claim the same row in
// the same delivery window.
func (r *Repository) ClaimPendingReportEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := make([]ports.OutboxEvent, 0, limit)
	err := r.db.WithTx(reportPlatformContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE report_outbox
			SET attempts=attempts+1,
			    next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second')
			WHERE id IN (
				SELECT id FROM report_outbox
				WHERE published_at IS NULL AND next_attempt_at<=now()
				ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1
			)
			RETURNING id, tenant_id, event_type, payload, created_at
		`, limit)
		if err != nil {
			return fmt.Errorf("report: claim outbox: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item ports.OutboxEvent
			if err := rows.Scan(&item.ID, &item.TenantID, &item.EventType, &item.Payload, &item.CreatedAt); err != nil {
				return fmt.Errorf("report: scan outbox: %w", err)
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (r *Repository) MarkReportEventPublished(ctx context.Context, id string) error {
	return r.markReportEvent(ctx, id, "", true)
}

func (r *Repository) MarkReportEventFailed(ctx context.Context, id, message string) error {
	return r.markReportEvent(ctx, id, message, false)
}

func (r *Repository) markReportEvent(ctx context.Context, id, message string, published bool) error {
	return r.db.WithTx(reportPlatformContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if published {
			_, err = tx.Exec(ctx, `UPDATE report_outbox SET published_at=now(), last_error=NULL WHERE id=$1`, id)
		} else {
			_, err = tx.Exec(ctx, `UPDATE report_outbox SET last_error=left($2,1000) WHERE id=$1`, id, message)
		}
		return err
	})
}

// RetryReportGeneration releases a failed lease with exponential backoff. At
// maxAttempts it marks the job failed and returns the card to draft so a human
// can correct the source data and explicitly retry.
func (r *Repository) RetryReportGeneration(ctx context.Context, job *domain.GenerationJob, message string, maxAttempts int) (bool, error) {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	terminal := job.Attempts >= maxAttempts
	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: job.TenantID})
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if terminal {
			if _, err := tx.Exec(ctx, `
				UPDATE report_generation_jobs
				SET status='failed', lease_expires_at=NULL, last_error=left($3,1000), updated_at=now()
				WHERE report_card_id=$1 AND tenant_id=$2 AND status='running'
			`, job.ReportCardID, job.TenantID, message); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE report_cards SET status='draft', updated_at=now() WHERE id=$1 AND tenant_id=$2 AND status='generating'`, job.ReportCardID, job.TenantID)
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE report_generation_jobs
			SET status='queued', lease_expires_at=NULL, last_error=left($3,1000),
			    next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second'),
			    updated_at=now()
			WHERE report_card_id=$1 AND tenant_id=$2 AND status='running'
		`, job.ReportCardID, job.TenantID, message)
		return err
	})
	if err != nil {
		return false, fmt.Errorf("report: retry generation job: %w", err)
	}
	return terminal, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanReportTemplate(row scanner) (*domain.ReportTemplate, error) {
	var t domain.ReportTemplate
	if err := row.Scan(
		&t.ID, &t.TenantID, &t.Name, &t.AcademicYearID, &t.BodyTemplate, &t.Status, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &t, nil
}

func scanReportCard(row scanner) (*domain.ReportCard, error) {
	var c domain.ReportCard
	if err := row.Scan(
		&c.ID, &c.TenantID, &c.StudentID, &c.AcademicYearID, &c.TermID, &c.TemplateID, &c.Status,
		&c.PDFPath, &c.GeneratedAt, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &c, nil
}

// nullIfEmpty maps an empty string to SQL NULL (nullable UUID columns).
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// isUniqueViolation reports whether err is a Postgres unique-constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
