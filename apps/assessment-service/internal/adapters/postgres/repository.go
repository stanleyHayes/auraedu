// Package postgres provides the Postgres implementation of the assessment-service repository.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/assessment-service/internal/ports"
	"github.com/auraedu/platform/db"
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

// NewRepository creates a Postgres-backed assessment repository.
func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

// --- Assessments. ---

func (r *Repository) CreateAssessment(ctx context.Context, tenantID string, a *domain.Assessment) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO assessments (id, tenant_id, academic_year_id, subject_id, type, title, description, max_score, due_date, status, class_ids, published_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::uuid[], $12, $13, $14)
		`, a.ID, tenantID, a.AcademicYearID, a.SubjectID, a.Type, a.Title, a.Description, a.MaxScore, a.DueDate, a.Status, classIDsOrEmpty(a.ClassIDs), a.PublishedAt, a.CreatedAt, a.UpdatedAt)
		if err != nil {
			return fmt.Errorf("assessment: create assessment: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetAssessmentByID(ctx context.Context, tenantID, id string) (*domain.Assessment, error) {
	var a *domain.Assessment
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, academic_year_id, subject_id, type, title, description, max_score, due_date, status, class_ids, published_at, created_at, updated_at
			FROM assessments
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, id, tenantID)
		got, err := scanAssessment(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("assessment: get assessment: %w", err)
		}
		a = got
		return nil
	})
	return a, err
}

func (r *Repository) ListAssessments(ctx context.Context, tenantID string, filter ports.AssessmentListFilter) ([]*domain.Assessment, string, error) {
	var out []*domain.Assessment
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listAssessmentsQuery(ctx, tx, tenantID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			rec, err := scanAssessment(rows)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("assessment: list assessments rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listAssessmentsQuery(ctx context.Context, tx pgx.Tx, tenantID string, filter ports.AssessmentListFilter) (pgx.Rows, error) {
	args := []any{tenantID}
	where := "tenant_id = $1 AND deleted_at IS NULL"

	if filter.AcademicYearID != "" {
		args = append(args, filter.AcademicYearID)
		where += fmt.Sprintf(" AND academic_year_id = $%d", len(args))
	}
	if filter.SubjectID != "" {
		args = append(args, filter.SubjectID)
		where += fmt.Sprintf(" AND subject_id = $%d", len(args))
	}
	if filter.Type != "" {
		args = append(args, filter.Type)
		where += fmt.Sprintf(" AND type = $%d", len(args))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM assessments WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, academic_year_id, subject_id, type, title, description, max_score, due_date, status, class_ids, published_at, created_at, updated_at
		FROM assessments
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *Repository) UpdateAssessment(ctx context.Context, tenantID string, a *domain.Assessment) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE assessments
			SET academic_year_id = $3, subject_id = $4, type = $5, title = $6, description = $7, max_score = $8, due_date = $9, status = $10, class_ids = $11::uuid[], published_at = $12, updated_at = $13
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, a.ID, tenantID, a.AcademicYearID, a.SubjectID, a.Type, a.Title, a.Description, a.MaxScore, a.DueDate, a.Status, classIDsOrEmpty(a.ClassIDs), a.PublishedAt, a.UpdatedAt)
		if err != nil {
			return fmt.Errorf("assessment: update assessment: %w", err)
		}
		return nil
	})
}

func (r *Repository) DeleteAssessment(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		now := time.Now().UTC()
		_, err := tx.Exec(ctx, `
			UPDATE assessments
			SET deleted_at = $3
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, id, tenantID, now)
		if err != nil {
			return fmt.Errorf("assessment: delete assessment: %w", err)
		}
		return nil
	})
}

// --- Scores. ---

func (r *Repository) CreateScore(ctx context.Context, tenantID string, s *domain.Score) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO scores (id, tenant_id, assessment_id, student_id, score, recorded_by, notes, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, s.ID, tenantID, s.AssessmentID, s.StudentID, s.Score, s.RecordedBy, s.Notes, s.CreatedAt, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("assessment: create score: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetScoreByID(ctx context.Context, tenantID, assessmentID, scoreID string) (*domain.Score, error) {
	var s *domain.Score
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, assessment_id, student_id, score, recorded_by, notes, created_at, updated_at
			FROM scores
			WHERE id = $1 AND assessment_id = $2 AND tenant_id = $3 AND deleted_at IS NULL
		`, scoreID, assessmentID, tenantID)
		got, err := scanScore(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("assessment: get score: %w", err)
		}
		s = got
		return nil
	})
	return s, err
}

func (r *Repository) ListScores(ctx context.Context, tenantID, assessmentID string, filter ports.ScoreListFilter) ([]*domain.Score, string, error) {
	var out []*domain.Score
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listScoresQuery(ctx, tx, tenantID, assessmentID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			rec, err := scanScore(rows)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("assessment: list scores rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listScoresQuery(ctx context.Context, tx pgx.Tx, tenantID, assessmentID string, filter ports.ScoreListFilter) (pgx.Rows, error) {
	args := []any{tenantID, assessmentID}
	where := "tenant_id = $1 AND assessment_id = $2 AND deleted_at IS NULL"

	if filter.StudentID != "" {
		args = append(args, filter.StudentID)
		where += fmt.Sprintf(" AND student_id = $%d", len(args))
	}

	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM scores WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, assessment_id, student_id, score, recorded_by, notes, created_at, updated_at
		FROM scores
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *Repository) UpdateScore(ctx context.Context, tenantID string, s *domain.Score) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE scores
			SET score = $3, recorded_by = $4, notes = $5, updated_at = $6
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, s.ID, tenantID, s.Score, s.RecordedBy, s.Notes, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("assessment: update score: %w", err)
		}
		return nil
	})
}

func (r *Repository) DeleteScore(ctx context.Context, tenantID, assessmentID, scoreID string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		now := time.Now().UTC()
		_, err := tx.Exec(ctx, `
			UPDATE scores
			SET deleted_at = $4
			WHERE id = $1 AND assessment_id = $2 AND tenant_id = $3 AND deleted_at IS NULL
		`, scoreID, assessmentID, tenantID, now)
		if err != nil {
			return fmt.Errorf("assessment: delete score: %w", err)
		}
		return nil
	})
}

// --- Assignments. ---

func (r *Repository) ListAssignments(ctx context.Context, tenantID string, filter ports.AssignmentListFilter) ([]*domain.Assessment, string, error) {
	var out []*domain.Assessment
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		args := []any{tenantID, string(domain.TypeAssignment)}
		where := "tenant_id = $1 AND type = $2 AND deleted_at IS NULL"

		if filter.SubjectID != "" {
			args = append(args, filter.SubjectID)
			where += fmt.Sprintf(" AND subject_id = $%d", len(args))
		}
		if filter.ClassID != "" {
			args = append(args, filter.ClassID)
			where += fmt.Sprintf(" AND class_ids @> ARRAY[$%d]::uuid[]", len(args))
		}
		if filter.StudentID != "" {
			args = append(args, filter.StudentID)
			where += fmt.Sprintf(` AND EXISTS (
				SELECT 1 FROM scores s
				WHERE s.tenant_id = $1 AND s.assessment_id = assessments.id AND s.student_id = $%d AND s.deleted_at IS NULL
			)`, len(args))
		}
		if filter.Status != "" {
			args = append(args, filter.Status)
			where += fmt.Sprintf(" AND status = $%d", len(args))
		}
		if filter.Cursor != "" {
			args = append(args, filter.Cursor)
			where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM assessments WHERE id = $%d AND tenant_id = $1)", len(args))
		}

		args = append(args, filter.Limit)
		sql := fmt.Sprintf(`
			SELECT id, tenant_id, academic_year_id, subject_id, type, title, description, max_score, due_date, status, class_ids, published_at, created_at, updated_at
			FROM assessments
			WHERE %s
			ORDER BY created_at ASC, id ASC
			LIMIT $%d
		`, where, len(args))
		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return fmt.Errorf("assessment: list assignments: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			rec, err := scanAssessment(rows)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("assessment: list assignments rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

// --- Gradebook. ---

func (r *Repository) GradebookScores(ctx context.Context, tenantID string, filter ports.GradebookFilter) ([]domain.GradeRow, error) {
	var out []domain.GradeRow
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		args := []any{tenantID}
		where := "s.tenant_id = $1 AND s.deleted_at IS NULL AND a.deleted_at IS NULL"

		if filter.StudentID != "" {
			args = append(args, filter.StudentID)
			where += fmt.Sprintf(" AND s.student_id = $%d", len(args))
		}
		if filter.ClassID != "" {
			args = append(args, filter.ClassID)
			where += fmt.Sprintf(" AND a.class_ids @> ARRAY[$%d]::uuid[]", len(args))
		}
		if filter.AcademicYearID != "" {
			args = append(args, filter.AcademicYearID)
			where += fmt.Sprintf(" AND a.academic_year_id = $%d", len(args))
		}
		if filter.SubjectID != "" {
			args = append(args, filter.SubjectID)
			where += fmt.Sprintf(" AND a.subject_id = $%d", len(args))
		}

		sql := fmt.Sprintf(`
			SELECT a.subject_id, s.score, a.max_score
			FROM scores s
			JOIN assessments a ON a.tenant_id = s.tenant_id AND a.id = s.assessment_id
			WHERE %s
			ORDER BY a.subject_id ASC, s.created_at ASC
		`, where)
		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return fmt.Errorf("assessment: gradebook scores: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var row domain.GradeRow
			if err := rows.Scan(&row.SubjectID, &row.Score, &row.MaxScore); err != nil {
				return err
			}
			out = append(out, row)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("assessment: gradebook scores rows: %w", err)
		}
		return nil
	})
	return out, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAssessment(row scanner) (*domain.Assessment, error) {
	var a domain.Assessment
	if err := row.Scan(
		&a.ID, &a.TenantID, &a.AcademicYearID, &a.SubjectID, &a.Type, &a.Title,
		&a.Description, &a.MaxScore, &a.DueDate, &a.Status, &a.ClassIDs, &a.PublishedAt, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &a, nil
}

// classIDsOrEmpty maps a nil slice to an empty one so the NOT NULL class_ids
// column never receives NULL.
func classIDsOrEmpty(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

func scanScore(row scanner) (*domain.Score, error) {
	var s domain.Score
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.AssessmentID, &s.StudentID, &s.Score,
		&s.RecordedBy, &s.Notes, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}
