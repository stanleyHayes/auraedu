package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/auraedu/report-service/internal/domain"
)

// ScoreRecordedInput carries the materializable fields of an
// assessment.score_recorded.v1 event. Producers may omit subject_id,
// max_score and term_id; the source key falls back assessment_id → score_id →
// event id so replays and corrections converge on one entry.
type ScoreRecordedInput struct {
	EventID      string
	TenantID     string
	StudentID    string
	SubjectID    string
	AssessmentID string
	ScoreID      string
	TermID       string
	Score        float64
	MaxScore     *float64
}

// AttendanceMarkedInput carries the materializable fields of an
// attendance.marked.v1 event. One entry per student day; re-marks rewrite it.
type AttendanceMarkedInput struct {
	EventID   string
	TenantID  string
	StudentID string
	Date      string
	Status    string
}

// MaterializeScore upserts the score carried by an assessment.score_recorded.v1
// event into the student's DRAFT report card for the period, auto-creating the
// draft when none exists. It is idempotent on the entry's natural key and a
// no-op when the report_cards feature is disabled for the tenant.
func (s *Service) MaterializeScore(ctx context.Context, in ScoreRecordedInput) error {
	if in.TenantID == "" {
		return domain.ErrMissingTenant
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, in.TenantID, FeatureReportCards) {
		return nil
	}
	card, err := s.draftReportCardFor(ctx, in.TenantID, in.StudentID, in.TermID)
	if err != nil {
		return err
	}
	entry, err := domain.NewScoreEntry(
		in.TenantID, card.ID, in.StudentID, in.SubjectID,
		firstNonEmpty(in.AssessmentID, in.ScoreID, in.EventID), in.EventID,
		in.Score, in.MaxScore,
	)
	if err != nil {
		return err
	}
	return s.repo.UpsertScoreEntry(ctx, in.TenantID, entry)
}

// MaterializeAttendance upserts the day status carried by an
// attendance.marked.v1 event into the student's DRAFT report card,
// auto-creating the draft when none exists. Idempotent per (card, date) and a
// no-op when the report_cards feature is disabled for the tenant.
func (s *Service) MaterializeAttendance(ctx context.Context, in AttendanceMarkedInput) error {
	if in.TenantID == "" {
		return domain.ErrMissingTenant
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, in.TenantID, FeatureReportCards) {
		return nil
	}
	card, err := s.draftReportCardFor(ctx, in.TenantID, in.StudentID, "")
	if err != nil {
		return err
	}
	entry, err := domain.NewAttendanceEntry(in.TenantID, card.ID, in.StudentID, in.Date, in.Status, in.EventID)
	if err != nil {
		return err
	}
	return s.repo.UpsertAttendanceEntry(ctx, in.TenantID, entry)
}

// draftReportCardFor finds the DRAFT report card for a student+period, creating
// one when none exists. Concurrent auto-creates race on a partial unique
// index; the loser re-reads the winner's card.
func (s *Service) draftReportCardFor(ctx context.Context, tenantID, studentID, termID string) (*domain.ReportCard, error) {
	card, err := s.repo.FindDraftReportCard(ctx, tenantID, studentID, termID)
	if err == nil {
		return card, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	draft, err := domain.NewEventDraftReportCard(tenantID, studentID, "", termID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateReportCard(ctx, tenantID, draft); err != nil {
		if !errors.Is(err, domain.ErrConflict) {
			return nil, err
		}
		card, rerr := s.repo.FindDraftReportCard(ctx, tenantID, studentID, termID)
		if rerr != nil {
			return nil, fmt.Errorf("report: re-read draft after create conflict: %w", rerr)
		}
		return card, nil
	}
	return draft, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
