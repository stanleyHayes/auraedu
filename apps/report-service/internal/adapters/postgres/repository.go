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
			INSERT INTO report_cards (id, tenant_id, student_id, academic_year_id, template_id, status, pdf_path, generated_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, c.ID, tenantID, c.StudentID, c.AcademicYearID, c.TemplateID, c.Status, c.PDFPath, c.GeneratedAt, c.CreatedAt, c.UpdatedAt)
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
			SELECT id, tenant_id, student_id, academic_year_id, template_id, status, pdf_path, generated_at, created_at, updated_at
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
		SELECT id, tenant_id, student_id, academic_year_id, template_id, status, pdf_path, generated_at, created_at, updated_at
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
			SET student_id = $3, academic_year_id = $4, template_id = $5, status = $6, pdf_path = $7, generated_at = $8, updated_at = $9
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, c.ID, tenantID, c.StudentID, c.AcademicYearID, c.TemplateID, c.Status, c.PDFPath, c.GeneratedAt, c.UpdatedAt)
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
		&c.ID, &c.TenantID, &c.StudentID, &c.AcademicYearID, &c.TemplateID, &c.Status,
		&c.PDFPath, &c.GeneratedAt, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &c, nil
}
