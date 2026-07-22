package domain

import "time"

// Transcript is a read model assembled from a learner's published and archived
// report cards. It is derived on request so corrected report-card evidence is
// reflected without maintaining a second mutable source of truth.
type Transcript struct {
	TenantID    string            `json:"tenant_id"`
	StudentID   string            `json:"student_id"`
	Entries     []TranscriptEntry `json:"entries"`
	GeneratedAt time.Time         `json:"generated_at"`
}

// TranscriptEntry captures one published academic period and its materialized
// subject results and attendance evidence.
type TranscriptEntry struct {
	ReportCardID   string            `json:"report_card_id"`
	AcademicYearID string            `json:"academic_year_id,omitempty"`
	TermID         string            `json:"term_id,omitempty"`
	PublishedAt    time.Time         `json:"published_at"`
	Scores         []SubjectScore    `json:"scores"`
	Attendance     AttendanceSummary `json:"attendance"`
}
