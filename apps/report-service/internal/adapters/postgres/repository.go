// Package postgres persists report aggregates in PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/platform/db"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
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

// NewRepository creates a Postgres-backed report repository.
func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

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
