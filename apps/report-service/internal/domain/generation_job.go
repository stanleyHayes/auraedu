package domain

import "time"

// GenerationJob is the durable work record used to publish one report-card PDF.
// ReportCardID is also the stable job identity, so replayed requests cannot create
// duplicate work for the same aggregate.
type GenerationJob struct {
	ReportCardID  string
	TenantID      string
	Attempts      int
	LeaseExpires  *time.Time
	NextAttemptAt time.Time
}
