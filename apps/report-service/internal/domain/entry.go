package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Attendance statuses mirrored from attendance.marked.v1.
const (
	AttendanceStatusPresent = "present"
	AttendanceStatusAbsent  = "absent"
	AttendanceStatusLate    = "late"
	AttendanceStatusExcused = "excused"
)

// ScoreEntry is a single score materialized from an assessment.score_recorded.v1
// event. SourceKey is the idempotency natural key: the assessment_id when the
// producer supplies it, otherwise the score_id or the event id. SubjectID may be
// empty when the producer omits it (the entry is then rendered per assessment).
type ScoreEntry struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	ReportCardID string    `json:"report_card_id"`
	StudentID    string    `json:"student_id"`
	SubjectID    string    `json:"subject_id,omitempty"`
	SourceKey    string    `json:"source_key"`
	Score        float64   `json:"score"`
	MaxScore     *float64  `json:"max_score,omitempty"`
	LastEventID  string    `json:"last_event_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewScoreEntry constructs a ScoreEntry, enforcing invariants.
func NewScoreEntry(tenantID, reportCardID, studentID, subjectID, sourceKey, eventID string, score float64, maxScore *float64) (*ScoreEntry, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(reportCardID) == "" {
		return nil, fmt.Errorf("%w: report_card_id is required", ErrValidation)
	}
	if strings.TrimSpace(studentID) == "" {
		return nil, fmt.Errorf("%w: student_id is required", ErrValidation)
	}
	if strings.TrimSpace(sourceKey) == "" {
		return nil, fmt.Errorf("%w: source_key is required", ErrValidation)
	}
	if strings.TrimSpace(eventID) == "" {
		return nil, fmt.Errorf("%w: event id is required", ErrValidation)
	}
	if score < 0 {
		return nil, fmt.Errorf("%w: score cannot be negative", ErrValidation)
	}
	if maxScore != nil && *maxScore < 0 {
		return nil, fmt.Errorf("%w: max_score cannot be negative", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("report: generate score entry id: %w", err)
	}
	now := time.Now().UTC()
	return &ScoreEntry{
		ID:           id.String(),
		TenantID:     tenantID,
		ReportCardID: strings.TrimSpace(reportCardID),
		StudentID:    strings.TrimSpace(studentID),
		SubjectID:    strings.TrimSpace(subjectID),
		SourceKey:    strings.TrimSpace(sourceKey),
		Score:        score,
		MaxScore:     maxScore,
		LastEventID:  strings.TrimSpace(eventID),
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// AttendanceEntry is one day's attendance status materialized from an
// attendance.marked.v1 event. The natural idempotency key is
// (report_card_id, Date): re-marks update the row in place.
type AttendanceEntry struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	ReportCardID string    `json:"report_card_id"`
	StudentID    string    `json:"student_id"`
	Date         time.Time `json:"date"`
	Status       string    `json:"status"`
	LastEventID  string    `json:"last_event_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewAttendanceEntry constructs an AttendanceEntry, enforcing invariants.
// date must be a calendar day in YYYY-MM-DD form.
func NewAttendanceEntry(tenantID, reportCardID, studentID, date, status, eventID string) (*AttendanceEntry, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(reportCardID) == "" {
		return nil, fmt.Errorf("%w: report_card_id is required", ErrValidation)
	}
	if strings.TrimSpace(studentID) == "" {
		return nil, fmt.Errorf("%w: student_id is required", ErrValidation)
	}
	if strings.TrimSpace(eventID) == "" {
		return nil, fmt.Errorf("%w: event id is required", ErrValidation)
	}
	day, err := time.Parse(time.DateOnly, strings.TrimSpace(date))
	if err != nil {
		return nil, fmt.Errorf("%w: date must be YYYY-MM-DD", ErrValidation)
	}
	s := strings.TrimSpace(status)
	if !isValidAttendanceStatus(s) {
		return nil, fmt.Errorf("%w: status must be present, absent, late or excused", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("report: generate attendance entry id: %w", err)
	}
	now := time.Now().UTC()
	return &AttendanceEntry{
		ID:           id.String(),
		TenantID:     tenantID,
		ReportCardID: strings.TrimSpace(reportCardID),
		StudentID:    strings.TrimSpace(studentID),
		Date:         day,
		Status:       s,
		LastEventID:  strings.TrimSpace(eventID),
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func isValidAttendanceStatus(s string) bool {
	switch s {
	case AttendanceStatusPresent, AttendanceStatusAbsent, AttendanceStatusLate, AttendanceStatusExcused:
		return true
	}
	return false
}

// SubjectScore aggregates a student's score entries for one subject (or for one
// assessment when the subject is unknown) for rendering.
type SubjectScore struct {
	Label    string   `json:"label"`
	Score    float64  `json:"score"`
	MaxScore *float64 `json:"max_score,omitempty"`
	Count    int      `json:"count"`
}

// Percentage returns the score as a percentage of MaxScore, or false when no
// max is known for every entry in the group.
func (s SubjectScore) Percentage() (float64, bool) {
	if s.MaxScore == nil || *s.MaxScore <= 0 {
		return 0, false
	}
	return s.Score / *s.MaxScore * 100, true
}

// AggregateScores groups score entries per subject, summing scores. Entries
// without a subject are grouped per assessment (source key) and labeled
// "Assessment <id>". MaxScore is summed only when every entry in the group has
// one. Groups are sorted by label for stable rendering.
func AggregateScores(entries []*ScoreEntry) []SubjectScore {
	type group struct {
		label  string
		score  float64
		max    float64
		hasMax bool
		allMax bool
		count  int
	}
	groups := map[string]*group{}
	var order []string
	for _, e := range entries {
		if e == nil {
			continue
		}
		key := e.SubjectID
		label := e.SubjectID
		if key == "" {
			key = "assessment:" + e.SourceKey
			label = "Assessment " + shortID(e.SourceKey)
		}
		g, ok := groups[key]
		if !ok {
			g = &group{label: label, hasMax: false, allMax: true}
			groups[key] = g
			order = append(order, key)
		}
		g.score += e.Score
		g.count++
		if e.MaxScore != nil {
			g.max += *e.MaxScore
			g.hasMax = true
		} else {
			g.allMax = false
		}
	}

	out := make([]SubjectScore, 0, len(groups))
	for _, key := range order {
		g := groups[key]
		ss := SubjectScore{Label: g.label, Score: g.score, Count: g.count}
		if g.hasMax && g.allMax {
			maximum := g.max
			ss.MaxScore = &maximum
		}
		out = append(out, ss)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

// AttendanceSummary aggregates attendance entries for rendering. Rate counts
// present and late days as attended.
type AttendanceSummary struct {
	Present int `json:"present"`
	Absent  int `json:"absent"`
	Late    int `json:"late"`
	Excused int `json:"excused"`
}

// Total returns the number of marked days.
func (s AttendanceSummary) Total() int { return s.Present + s.Absent + s.Late + s.Excused }

// Rate returns the attendance rate in percent ((present+late)/total*100), or
// false when no days are marked.
func (s AttendanceSummary) Rate() (float64, bool) {
	if s.Total() == 0 {
		return 0, false
	}
	return float64(s.Present+s.Late) / float64(s.Total()) * 100, true
}

// SummarizeAttendance counts attendance entries per status.
func SummarizeAttendance(entries []*AttendanceEntry) AttendanceSummary {
	var s AttendanceSummary
	for _, e := range entries {
		if e == nil {
			continue
		}
		switch e.Status {
		case AttendanceStatusPresent:
			s.Present++
		case AttendanceStatusAbsent:
			s.Absent++
		case AttendanceStatusLate:
			s.Late++
		case AttendanceStatusExcused:
			s.Excused++
		}
	}
	return s
}

// ReportCardDocument is the render model for the PDF generator: the card, its
// (optional) template, aggregated scores and the attendance summary.
type ReportCardDocument struct {
	Card        *ReportCard
	Template    *ReportTemplate // nil when the card has no template assigned
	Scores      []SubjectScore
	Attendance  AttendanceSummary
	GeneratedAt time.Time
}

// shortID trims an id for display (first 8 chars).
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
