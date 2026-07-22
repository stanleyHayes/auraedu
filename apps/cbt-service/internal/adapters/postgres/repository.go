// Package postgres provides the Postgres implementation of the CBT repository port.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/auraedu/cbt-service/internal/domain"
	"github.com/auraedu/cbt-service/internal/ports"
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

// NewRepository creates a Postgres-backed CBT repository.
func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

// --- Question banks. ---

func (r *Repository) CreateQuestion(ctx context.Context, tenantID string, q *domain.QuestionBank) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		options, err := json.Marshal(q.Options)
		if err != nil {
			return fmt.Errorf("cbt: marshal options: %w", err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO cbt_questions (
				id, tenant_id, academic_year_id, subject_id, question_text, question_type,
				options, correct_answer, marks, status, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, q.ID, tenantID, q.AcademicYearID, q.SubjectID, q.QuestionText, q.QuestionType, options, q.CorrectAnswer, q.Marks, q.Status, q.CreatedAt, q.UpdatedAt)
		if err != nil {
			return fmt.Errorf("cbt: create question: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetQuestionByID(ctx context.Context, tenantID, id string) (*domain.QuestionBank, error) {
	var q *domain.QuestionBank
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, academic_year_id, subject_id, question_text, question_type, options, correct_answer, marks, status, created_at, updated_at
			FROM cbt_questions
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, id, tenantID)
		got, err := scanQuestion(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("cbt: get question: %w", err)
		}
		q = got
		return nil
	})
	return q, err
}

func (r *Repository) ListQuestions(ctx context.Context, tenantID string, filter ports.QuestionListFilter) ([]*domain.QuestionBank, string, error) {
	var out []*domain.QuestionBank
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listQuestionsQuery(ctx, tx, tenantID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			rec, err := scanQuestion(rows)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("cbt: list questions rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listQuestionsQuery(ctx context.Context, tx pgx.Tx, tenantID string, filter ports.QuestionListFilter) (pgx.Rows, error) {
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
	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM cbt_questions WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, academic_year_id, subject_id, question_text, question_type, options, correct_answer, marks, status, created_at, updated_at
		FROM cbt_questions
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *Repository) UpdateQuestion(ctx context.Context, tenantID string, q *domain.QuestionBank) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		options, err := json.Marshal(q.Options)
		if err != nil {
			return fmt.Errorf("cbt: marshal options: %w", err)
		}
		_, err = tx.Exec(ctx, `
			UPDATE cbt_questions
			SET academic_year_id = $3, subject_id = $4, question_text = $5,
			    question_type = $6, options = $7, correct_answer = $8,
			    marks = $9, status = $10, updated_at = $11
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, q.ID, tenantID, q.AcademicYearID, q.SubjectID, q.QuestionText, q.QuestionType, options, q.CorrectAnswer, q.Marks, q.Status, q.UpdatedAt)
		if err != nil {
			return fmt.Errorf("cbt: update question: %w", err)
		}
		return nil
	})
}

func (r *Repository) DeleteQuestion(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		now := time.Now().UTC()
		_, err := tx.Exec(ctx, `
			UPDATE cbt_questions
			SET deleted_at = $3
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, id, tenantID, now)
		if err != nil {
			return fmt.Errorf("cbt: delete question: %w", err)
		}
		return nil
	})
}

// --- Exam sessions. ---

func (r *Repository) CreateExamSession(ctx context.Context, tenantID string, e *domain.ExamSession) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		qids, err := stringSliceToUUIDSlice(e.QuestionIDs)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO cbt_exam_sessions (
				id, tenant_id, title, academic_year_id, subject_id, question_ids,
				duration_minutes, start_at, end_at, status, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, e.ID, tenantID, e.Title, e.AcademicYearID, e.SubjectID, qids, e.DurationMinutes, e.StartAt, e.EndAt, e.Status, e.CreatedAt, e.UpdatedAt)
		if err != nil {
			return fmt.Errorf("cbt: create exam session: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetExamSessionByID(ctx context.Context, tenantID, id string) (*domain.ExamSession, error) {
	var e *domain.ExamSession
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, title, academic_year_id, subject_id, question_ids, duration_minutes, start_at, end_at, status, created_at, updated_at
			FROM cbt_exam_sessions
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, id, tenantID)
		got, err := scanExamSession(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("cbt: get exam session: %w", err)
		}
		e = got
		return nil
	})
	return e, err
}

func (r *Repository) ListExamSessions(ctx context.Context, tenantID string, filter ports.ExamSessionListFilter) ([]*domain.ExamSession, string, error) {
	var out []*domain.ExamSession
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listExamSessionsQuery(ctx, tx, tenantID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			rec, err := scanExamSession(rows)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("cbt: list exam sessions rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listExamSessionsQuery(ctx context.Context, tx pgx.Tx, tenantID string, filter ports.ExamSessionListFilter) (pgx.Rows, error) {
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
	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM cbt_exam_sessions WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, title, academic_year_id, subject_id, question_ids, duration_minutes, start_at, end_at, status, created_at, updated_at
		FROM cbt_exam_sessions
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *Repository) UpdateExamSession(ctx context.Context, tenantID string, e *domain.ExamSession) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		qids, err := stringSliceToUUIDSlice(e.QuestionIDs)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE cbt_exam_sessions
			SET title = $3, academic_year_id = $4, subject_id = $5, question_ids = $6, duration_minutes = $7, start_at = $8, end_at = $9, status = $10, updated_at = $11
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, e.ID, tenantID, e.Title, e.AcademicYearID, e.SubjectID, qids, e.DurationMinutes, e.StartAt, e.EndAt, e.Status, e.UpdatedAt)
		if err != nil {
			return fmt.Errorf("cbt: update exam session: %w", err)
		}
		return nil
	})
}

func (r *Repository) DeleteExamSession(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		now := time.Now().UTC()
		_, err := tx.Exec(ctx, `
			UPDATE cbt_exam_sessions
			SET deleted_at = $3
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, id, tenantID, now)
		if err != nil {
			return fmt.Errorf("cbt: delete exam session: %w", err)
		}
		return nil
	})
}

// --- Submissions. ---

func (r *Repository) CreateSubmission(ctx context.Context, tenantID string, s *domain.Submission) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		answers, err := json.Marshal(s.Answers)
		if err != nil {
			return fmt.Errorf("cbt: marshal answers: %w", err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO cbt_submissions (id, tenant_id, exam_session_id, student_id, answers, status, score, max_score, submitted_at, graded_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, s.ID, tenantID, s.ExamSessionID, s.StudentID, answers, s.Status, s.Score, s.MaxScore, s.SubmittedAt, s.GradedAt, s.CreatedAt, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("cbt: create submission: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetSubmissionByID(ctx context.Context, tenantID, id string) (*domain.Submission, error) {
	var s *domain.Submission
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, exam_session_id, student_id, answers, status, score, max_score, submitted_at, graded_at, created_at, updated_at
			FROM cbt_submissions
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, id, tenantID)
		got, err := scanSubmission(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("cbt: get submission: %w", err)
		}
		s = got
		return nil
	})
	return s, err
}

func (r *Repository) ListSubmissions(ctx context.Context, tenantID string, filter ports.SubmissionListFilter) ([]*domain.Submission, string, error) {
	var out []*domain.Submission
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listSubmissionsQuery(ctx, tx, tenantID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			rec, err := scanSubmission(rows)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("cbt: list submissions rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listSubmissionsQuery(ctx context.Context, tx pgx.Tx, tenantID string, filter ports.SubmissionListFilter) (pgx.Rows, error) {
	args := []any{tenantID}
	where := "tenant_id = $1 AND deleted_at IS NULL"

	if filter.ExamSessionID != "" {
		args = append(args, filter.ExamSessionID)
		where += fmt.Sprintf(" AND exam_session_id = $%d", len(args))
	}
	if filter.StudentID != "" {
		args = append(args, filter.StudentID)
		where += fmt.Sprintf(" AND student_id = $%d", len(args))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM cbt_submissions WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, exam_session_id, student_id, answers, status, score, max_score, submitted_at, graded_at, created_at, updated_at
		FROM cbt_submissions
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *Repository) GetSubmissionByExamAndStudent(ctx context.Context, tenantID, examSessionID, studentID string) (*domain.Submission, error) {
	var s *domain.Submission
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, exam_session_id, student_id, answers, status, score, max_score, submitted_at, graded_at, created_at, updated_at
			FROM cbt_submissions
			WHERE exam_session_id = $1 AND student_id = $2 AND tenant_id = $3 AND deleted_at IS NULL
		`, examSessionID, studentID, tenantID)
		got, err := scanSubmission(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("cbt: get submission by exam/student: %w", err)
		}
		s = got
		return nil
	})
	return s, err
}

func (r *Repository) UpdateSubmission(ctx context.Context, tenantID string, s *domain.Submission) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		answers, err := json.Marshal(s.Answers)
		if err != nil {
			return fmt.Errorf("cbt: marshal answers: %w", err)
		}
		_, err = tx.Exec(ctx, `
			UPDATE cbt_submissions
			SET exam_session_id = $3, student_id = $4, answers = $5, status = $6, score = $7, max_score = $8, submitted_at = $9, graded_at = $10, updated_at = $11
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, s.ID, tenantID, s.ExamSessionID, s.StudentID, answers, s.Status, s.Score, s.MaxScore, s.SubmittedAt, s.GradedAt, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("cbt: update submission: %w", err)
		}
		return nil
	})
}

func (r *Repository) DeleteSubmission(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		now := time.Now().UTC()
		_, err := tx.Exec(ctx, `
			UPDATE cbt_submissions
			SET deleted_at = $3
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, id, tenantID, now)
		if err != nil {
			return fmt.Errorf("cbt: delete submission: %w", err)
		}
		return nil
	})
}

// --- Scanners and helpers. ---

type scanner interface {
	Scan(dest ...any) error
}

func scanQuestion(row scanner) (*domain.QuestionBank, error) {
	var q domain.QuestionBank
	var options []byte
	if err := row.Scan(
		&q.ID, &q.TenantID, &q.AcademicYearID, &q.SubjectID, &q.QuestionText, &q.QuestionType,
		&options, &q.CorrectAnswer, &q.Marks, &q.Status, &q.CreatedAt, &q.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if len(options) > 0 {
		if err := json.Unmarshal(options, &q.Options); err != nil {
			return nil, fmt.Errorf("cbt: unmarshal options: %w", err)
		}
	}
	return &q, nil
}

func scanExamSession(row scanner) (*domain.ExamSession, error) {
	var e domain.ExamSession
	var qids []uuid.UUID
	if err := row.Scan(
		&e.ID, &e.TenantID, &e.Title, &e.AcademicYearID, &e.SubjectID, &qids,
		&e.DurationMinutes, &e.StartAt, &e.EndAt, &e.Status, &e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		return nil, err
	}
	e.QuestionIDs = uuidSliceToStringSlice(qids)
	return &e, nil
}

func scanSubmission(row scanner) (*domain.Submission, error) {
	var s domain.Submission
	var answers []byte
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.ExamSessionID, &s.StudentID, &answers,
		&s.Status, &s.Score, &s.MaxScore, &s.SubmittedAt, &s.GradedAt, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	s.Answers = make(map[string]string)
	if len(answers) > 0 {
		if err := json.Unmarshal(answers, &s.Answers); err != nil {
			return nil, fmt.Errorf("cbt: unmarshal answers: %w", err)
		}
	}
	return &s, nil
}

func stringSliceToUUIDSlice(ids []string) ([]uuid.UUID, error) {
	arr := make([]uuid.UUID, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		u, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("cbt: invalid uuid %q: %w", id, err)
		}
		arr = append(arr, u)
	}
	return arr, nil
}

func uuidSliceToStringSlice(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, u := range ids {
		out = append(out, u.String())
	}
	return out
}

func (r *Repository) CommitCBTLifecycle(ctx context.Context, tenantID string, mutation ports.LifecycleMutation, events []ports.LifecycleEvent) error {
	if len(events) == 0 {
		return errors.New("cbt: lifecycle events are required")
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := applyCBTMutation(ctx, tx, tenantID, mutation); err != nil {
			return err
		}
		return enqueueCBTEvents(ctx, tx, tenantID, events)
	})
}

func applyCBTMutation(ctx context.Context, tx pgx.Tx, tenantID string, mutation ports.LifecycleMutation) error {
	switch mutation.Kind {
	case ports.CBTMutationQuestionCreate, ports.CBTMutationQuestionUpdate, ports.CBTMutationQuestionDelete:
		return applyQuestionMutation(ctx, tx, tenantID, mutation)
	case ports.CBTMutationExamCreate, ports.CBTMutationExamUpdate, ports.CBTMutationExamDelete:
		return applyExamMutation(ctx, tx, tenantID, mutation)
	case ports.CBTMutationSubmissionUpdate:
		return applySubmissionMutation(ctx, tx, tenantID, mutation.Submission)
	default:
		return fmt.Errorf("cbt: unsupported lifecycle mutation %q", mutation.Kind)
	}
}

func applyQuestionMutation(ctx context.Context, tx pgx.Tx, tenantID string, mutation ports.LifecycleMutation) error {
	question := mutation.Question
	if question == nil {
		return errors.New("cbt: question lifecycle requires question")
	}
	if mutation.Kind == ports.CBTMutationQuestionDelete {
		_, err := tx.Exec(ctx, `
			UPDATE cbt_questions SET deleted_at=now()
			WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, question.ID, tenantID)
		return err
	}
	options, err := json.Marshal(question.Options)
	if err != nil {
		return err
	}
	if mutation.Kind == ports.CBTMutationQuestionCreate {
		_, err = tx.Exec(ctx, `
			INSERT INTO cbt_questions(
				id,tenant_id,academic_year_id,subject_id,question_text,question_type,
				options,correct_answer,marks,status,created_at,updated_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			question.ID, tenantID, question.AcademicYearID, question.SubjectID,
			question.QuestionText, question.QuestionType, options, question.CorrectAnswer,
			question.Marks, question.Status, question.CreatedAt, question.UpdatedAt)
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE cbt_questions SET academic_year_id=$3,subject_id=$4,
			question_text=$5,question_type=$6,options=$7,correct_answer=$8,
			marks=$9,status=$10,updated_at=$11
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`,
		question.ID, tenantID, question.AcademicYearID, question.SubjectID,
		question.QuestionText, question.QuestionType, options, question.CorrectAnswer,
		question.Marks, question.Status, question.UpdatedAt)
	return err
}

func applyExamMutation(ctx context.Context, tx pgx.Tx, tenantID string, mutation ports.LifecycleMutation) error {
	exam := mutation.Exam
	if exam == nil {
		return errors.New("cbt: exam lifecycle requires exam")
	}
	if mutation.Kind == ports.CBTMutationExamDelete {
		_, err := tx.Exec(ctx, `
			UPDATE cbt_exam_sessions SET deleted_at=now()
			WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, exam.ID, tenantID)
		return err
	}
	questionIDs, err := stringSliceToUUIDSlice(exam.QuestionIDs)
	if err != nil {
		return err
	}
	if mutation.Kind == ports.CBTMutationExamCreate {
		_, err = tx.Exec(ctx, `
			INSERT INTO cbt_exam_sessions(
				id,tenant_id,title,academic_year_id,subject_id,question_ids,
				duration_minutes,start_at,end_at,status,created_at,updated_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			exam.ID, tenantID, exam.Title, exam.AcademicYearID, exam.SubjectID,
			questionIDs, exam.DurationMinutes, exam.StartAt, exam.EndAt,
			exam.Status, exam.CreatedAt, exam.UpdatedAt)
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE cbt_exam_sessions SET title=$3,academic_year_id=$4,subject_id=$5,
			question_ids=$6,duration_minutes=$7,start_at=$8,end_at=$9,
			status=$10,updated_at=$11
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`,
		exam.ID, tenantID, exam.Title, exam.AcademicYearID, exam.SubjectID,
		questionIDs, exam.DurationMinutes, exam.StartAt, exam.EndAt, exam.Status, exam.UpdatedAt)
	return err
}

func applySubmissionMutation(ctx context.Context, tx pgx.Tx, tenantID string, submission *domain.Submission) error {
	if submission == nil {
		return errors.New("cbt: submission lifecycle requires submission")
	}
	answers, err := json.Marshal(submission.Answers)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE cbt_submissions SET exam_session_id=$3,student_id=$4,answers=$5,
			status=$6,score=$7,max_score=$8,submitted_at=$9,graded_at=$10,updated_at=$11
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`,
		submission.ID, tenantID, submission.ExamSessionID, submission.StudentID,
		answers, submission.Status, submission.Score, submission.MaxScore,
		submission.SubmittedAt, submission.GradedAt, submission.UpdatedAt)
	return err
}

func enqueueCBTEvents(ctx context.Context, tx pgx.Tx, tenantID string, events []ports.LifecycleEvent) error {
	for _, event := range events {
		payload, err := json.Marshal(event.Payload)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO cbt_outbox(id,tenant_id,event_type,payload)
			VALUES($1,$2,$3,$4)`, uuid.NewString(), tenantID, event.EventType, payload); err != nil {
			return fmt.Errorf("cbt: enqueue lifecycle event: %w", err)
		}
	}
	return nil
}

func cbtOutboxContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__cbt_outbox__"})
}

func (r *Repository) ClaimPendingCBTEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := make([]ports.OutboxEvent, 0, limit)
	err := r.db.WithTx(cbtOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE cbt_outbox SET attempts=attempts+1,
				next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second')
			WHERE id IN(
				SELECT id FROM cbt_outbox
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
func (r *Repository) MarkCBTEventPublished(ctx context.Context, id string) error {
	return r.db.WithTx(cbtOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE cbt_outbox SET published_at=now(),last_error=NULL WHERE id=$1`, id)
		return err
	})
}
func (r *Repository) MarkCBTEventFailed(ctx context.Context, id, message string) error {
	return r.db.WithTx(cbtOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE cbt_outbox SET last_error=left($2,1000) WHERE id=$1`, id, message)
		return err
	})
}
