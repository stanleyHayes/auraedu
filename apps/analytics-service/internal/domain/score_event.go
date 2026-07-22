package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	ScoreRecorded = "recorded"
	ScoreUpdated  = "updated"
	ScoreDeleted  = "deleted"
)

// AssessmentScoreEvent is the complete current-state fact carried by score
// lifecycle CloudEvents. Analytics never reads the Assessment database.
type AssessmentScoreEvent struct {
	EventID        string
	EventType      string
	Operation      string
	ScoreID        string
	AssessmentID   string
	StudentID      string
	SubjectID      string
	AcademicYearID string
	Score          float64
	MaxScore       float64
	RecordedAt     time.Time
	OccurredAt     time.Time
}

func (event AssessmentScoreEvent) Validate() error {
	for name, value := range map[string]string{
		"event_id":         event.EventID,
		"event_type":       event.EventType,
		"score_id":         event.ScoreID,
		"assessment_id":    event.AssessmentID,
		"student_id":       event.StudentID,
		"subject_id":       event.SubjectID,
		"academic_year_id": event.AcademicYearID,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: score event %s is required", ErrValidation, name)
		}
	}
	if event.Operation != ScoreRecorded && event.Operation != ScoreUpdated && event.Operation != ScoreDeleted {
		return fmt.Errorf("%w: unsupported score operation %q", ErrValidation, event.Operation)
	}
	if event.MaxScore <= 0 || event.Score < 0 || event.Score > event.MaxScore {
		return fmt.Errorf("%w: score must be between zero and max_score", ErrValidation)
	}
	if event.RecordedAt.IsZero() {
		return fmt.Errorf("%w: score event recorded_at is required", ErrValidation)
	}
	return nil
}

func (event AssessmentScoreEvent) BucketDate() string {
	return event.RecordedAt.UTC().Format(time.DateOnly)
}

func (event AssessmentScoreEvent) Dimensions() Dimensions {
	return Dimensions{
		"student_id":       event.StudentID,
		"subject_id":       event.SubjectID,
		"academic_year_id": event.AcademicYearID,
	}
}
