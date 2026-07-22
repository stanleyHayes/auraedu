package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type TimetableRepository struct{ db *db.DB }

func NewTimetableRepository(database *db.DB) *TimetableRepository {
	return &TimetableRepository{db: database}
}

func (r *TimetableRepository) Create(ctx context.Context, tenantID string, entry *domain.TimetableEntry) error {
	start, end := minute(entry.StartTime), minute(entry.EndTime)
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO timetable_entries (
				id, tenant_id, class_id, term_id, subject_id, teacher_id, weekday,
				start_minute, end_minute, room, status, created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			entry.ID, tenantID, entry.ClassID, entry.TermID, entry.SubjectID,
			entry.TeacherID, entry.Weekday, start, end, entry.Room, entry.Status,
			entry.CreatedAt, entry.UpdatedAt,
		)
		return timetableWriteError(err)
	})
}

func (r *TimetableRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.TimetableEntry, error) {
	var result *domain.TimetableEntry
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		entry, err := scanTimetable(tx.QueryRow(ctx, timetableSelect+` WHERE id=$1 AND tenant_id=$2`, id, tenantID))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("academic: get timetable: %w", err)
		}
		result = entry
		return nil
	})
	return result, err
}

func (r *TimetableRepository) List(ctx context.Context, tenantID string, filter ports.TimetableFilter) ([]*domain.TimetableEntry, error) {
	var result []*domain.TimetableEntry
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		args := []any{tenantID}
		where := "tenant_id=$1"
		if filter.ClassIDs != nil {
			if len(filter.ClassIDs) == 0 {
				where += " AND false"
			} else {
				args = append(args, filter.ClassIDs)
				where += fmt.Sprintf(" AND class_id=ANY($%d)", len(args))
			}
		}
		if filter.TermID != "" {
			args = append(args, filter.TermID)
			where += fmt.Sprintf(" AND term_id=$%d", len(args))
		}
		if filter.Weekday != 0 {
			args = append(args, filter.Weekday)
			where += fmt.Sprintf(" AND weekday=$%d", len(args))
		}
		if filter.Status != "" {
			args = append(args, filter.Status)
			where += fmt.Sprintf(" AND status=$%d", len(args))
		}
		args = append(args, filter.Limit)
		rows, err := tx.Query(ctx, timetableSelect+fmt.Sprintf(` WHERE %s ORDER BY weekday,start_minute,id LIMIT $%d`, where, len(args)), args...)
		if err != nil {
			return fmt.Errorf("academic: list timetable: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			entry, err := scanTimetable(rows)
			if err != nil {
				return err
			}
			result = append(result, entry)
		}
		return rows.Err()
	})
	if result == nil {
		result = []*domain.TimetableEntry{}
	}
	return result, err
}

func (r *TimetableRepository) Update(ctx context.Context, tenantID string, entry *domain.TimetableEntry) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `
			UPDATE timetable_entries
			SET teacher_id=$3, weekday=$4, start_minute=$5, end_minute=$6,
				room=$7, status=$8, updated_at=$9
			WHERE id=$1 AND tenant_id=$2`,
			entry.ID, tenantID, entry.TeacherID, entry.Weekday,
			minute(entry.StartTime), minute(entry.EndTime), entry.Room,
			entry.Status, entry.UpdatedAt,
		)
		if err = timetableWriteError(err); err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *TimetableRepository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `DELETE FROM timetable_entries WHERE id=$1 AND tenant_id=$2`, id, tenantID)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

const timetableSelect = `
	SELECT id, tenant_id, class_id, term_id, subject_id, teacher_id, weekday,
		start_minute, end_minute, room, status, created_at, updated_at
	FROM timetable_entries`

type timetableScanner interface{ Scan(...any) error }

func scanTimetable(row timetableScanner) (*domain.TimetableEntry, error) {
	var e domain.TimetableEntry
	var start, end int
	if err := row.Scan(
		&e.ID, &e.TenantID, &e.ClassID, &e.TermID, &e.SubjectID,
		&e.TeacherID, &e.Weekday, &start, &end, &e.Room, &e.Status,
		&e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		return nil, err
	}
	e.StartTime = formatMinute(start)
	e.EndTime = formatMinute(end)
	return &e, nil
}
func minute(value string) int {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0
	}
	return parsed.Hour()*60 + parsed.Minute()
}
func formatMinute(value int) string { return fmt.Sprintf("%02d:%02d", value/60, value%60) }
func timetableWriteError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23P01" || pgErr.Code == "23505") {
		return domain.ErrConflict
	}
	return fmt.Errorf("academic: write timetable: %w", err)
}
