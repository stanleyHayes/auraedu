package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/auraedu/analytics-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
)

// Projection is the analytics use case that materializes time-bucketed metrics
// from CloudEvents emitted by other domain services.
type Projection struct {
	repo ports.Repository
	log  *slog.Logger
}

// NewProjection creates the projection use case.
func NewProjection(repo ports.Repository, log *slog.Logger) *Projection {
	if log == nil {
		log = slog.Default()
	}
	return &Projection{repo: repo, log: log}
}

// ProcessEvent ingests a CloudEvent and updates the corresponding metric buckets.
// Unknown event types are logged and ignored.
func (p *Projection) ProcessEvent(ctx context.Context, event tenancy.CloudEvent) error {
	tenantID := event.TenantID
	if tenantID == "" {
		return fmt.Errorf("%w: event missing tenant_id", domain.ErrValidation)
	}

	bucketDate := time.Now().UTC().Format(time.DateOnly)
	if event.Time != "" {
		if t, err := time.Parse(time.RFC3339, event.Time); err == nil {
			bucketDate = t.UTC().Format(time.DateOnly)
		}
	}

	switch event.Type {
	case "lead.created.v1":
		return p.handleGrowthEvent(ctx, tenantID, bucketDate, event, domain.GrowthLeads)

	case "application.started.v1":
		return p.handleGrowthEvent(ctx, tenantID, bucketDate, event, domain.GrowthApplicationsStarted)

	case "application.submitted.v1":
		return p.handleGrowthEvent(ctx, tenantID, bucketDate, event, domain.GrowthApplicationsDone)

	case "application.admitted.v1":
		return p.handleGrowthEvent(ctx, tenantID, bucketDate, event, domain.GrowthAdmitted)

	case "offer.issued.v1":
		return p.handleGrowthEvent(ctx, tenantID, bucketDate, event, domain.GrowthOffersIssued)

	case "offer.accepted.v1":
		return p.handleGrowthEvent(ctx, tenantID, bucketDate, event, domain.GrowthOffersAccepted)

	case "student.enrolled.v1":
		return p.handleStudentCountEvent(ctx, tenantID, bucketDate, event, "students.count")

	case "attendance.marked.v1":
		return p.handleAttendanceMarked(ctx, tenantID, bucketDate, event)

	case "assessment.score_recorded.v1", "assessment.score_updated.v1", "assessment.score_deleted.v1":
		return p.handleAssessmentScore(ctx, tenantID, event)

	case "payment.received.v1":
		return p.handleSumEvent(ctx, tenantID, "payments.total", bucketDate, event, "amount")

	case "invoice.created.v1":
		return p.handleSumEvent(ctx, tenantID, "invoices.total", bucketDate, event, "amount")

	case "report.published.v1":
		return p.handleStudentCountEvent(ctx, tenantID, bucketDate, event, "reports.count")

	default:
		p.log.Info("ignoring unsupported event type", "type", event.Type, "id", event.ID)
		return nil
	}
}

func (p *Projection) handleGrowthEvent(ctx context.Context, tenantID, bucketDate string, event tenancy.CloudEvent, stage string) error {
	var payload struct {
		LeadID        string `json:"lead_id"`
		ApplicationID string `json:"application_id"`
		ProgrammeID   string `json:"programme_id"`
		IntakeID      string `json:"intake_id"`
		Source        string `json:"source"`
		CampaignID    string `json:"campaign_id"`
		CreatedAt     string `json:"created_at"`
		StartedAt     string `json:"started_at"`
		SubmittedAt   string `json:"submitted_at"`
		ReviewedAt    string `json:"reviewed_at"`
		IssuedAt      string `json:"issued_at"`
		AcceptedAt    string `json:"accepted_at"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("%w: decode %s payload: %w", domain.ErrValidation, event.Type, err)
	}
	if event.ID == "" {
		return fmt.Errorf("%w: growth event missing id", domain.ErrValidation)
	}
	if stage == domain.GrowthLeads && payload.LeadID == "" {
		return fmt.Errorf("%w: lead.created missing lead_id", domain.ErrValidation)
	}
	if stage != domain.GrowthLeads && (payload.ApplicationID == "" || payload.ProgrammeID == "") {
		return fmt.Errorf("%w: %s missing application_id or programme_id", domain.ErrValidation, event.Type)
	}
	if stage == domain.GrowthLeads && strings.TrimSpace(payload.Source) == "" {
		payload.Source = "unknown"
	}
	occurredAt := time.Now().UTC()
	for _, raw := range []string{payload.AcceptedAt, payload.IssuedAt, payload.ReviewedAt, payload.SubmittedAt, payload.StartedAt, payload.CreatedAt, event.Time} {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			occurredAt = parsed.UTC()
			break
		}
	}
	return p.repo.ApplyGrowthEvent(ctx, tenantID, domain.GrowthEvent{
		EventID: event.ID, EventType: event.Type, Stage: stage, BucketDate: bucketDate,
		LeadID: payload.LeadID, ApplicationID: payload.ApplicationID, ProgrammeID: payload.ProgrammeID,
		IntakeID: payload.IntakeID, Source: strings.ToLower(strings.TrimSpace(payload.Source)), CampaignID: payload.CampaignID,
		OccurredAt: occurredAt,
	})
}

func (p *Projection) handleAttendanceMarked(ctx context.Context, tenantID, bucketDate string, event tenancy.CloudEvent) error {
	var payload struct {
		StudentID string `json:"student_id"`
		Status    string `json:"status"`
	}
	if len(event.Data) > 0 {
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			p.log.Warn("attendance.marked.v1: failed to unmarshal payload", "err", err)
		}
	}
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	if strings.TrimSpace(payload.StudentID) == "" {
		return fmt.Errorf("%w: attendance.marked.v1 missing student_id", domain.ErrValidation)
	}
	dims := domain.Dimensions{"student_id": payload.StudentID}
	switch status {
	case "present", "absent", "late", "excused":
		name := fmt.Sprintf("attendance.%s", status)
		return p.applyCountEvent(ctx, tenantID, bucketDate, event, name, dims)
	default:
		p.log.Info("attendance.marked.v1: unknown status", "status", payload.Status)
		return nil
	}
}

func (p *Projection) handleAssessmentScore(ctx context.Context, tenantID string, event tenancy.CloudEvent) error {
	var payload struct {
		ScoreID        string  `json:"score_id"`
		AssessmentID   string  `json:"assessment_id"`
		StudentID      string  `json:"student_id"`
		Score          float64 `json:"score"`
		MaxScore       float64 `json:"max_score"`
		SubjectID      string  `json:"subject_id"`
		AcademicYearID string  `json:"academic_year_id"`
		RecordedAt     string  `json:"recorded_at"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("%w: decode %s payload: %w", domain.ErrValidation, event.Type, err)
	}
	recordedAt, err := time.Parse(time.RFC3339, payload.RecordedAt)
	if err != nil {
		return fmt.Errorf("%w: %s recorded_at must be RFC3339", domain.ErrValidation, event.Type)
	}
	operation := domain.ScoreRecorded
	switch event.Type {
	case "assessment.score_updated.v1":
		operation = domain.ScoreUpdated
	case "assessment.score_deleted.v1":
		operation = domain.ScoreDeleted
	}
	occurredAt := time.Now().UTC()
	if event.Time != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, event.Time); parseErr == nil {
			occurredAt = parsed.UTC()
		}
	}
	return p.repo.ApplyAssessmentScoreEvent(ctx, tenantID, domain.AssessmentScoreEvent{
		EventID: event.ID, EventType: event.Type, Operation: operation,
		ScoreID: payload.ScoreID, AssessmentID: payload.AssessmentID,
		StudentID: payload.StudentID, SubjectID: payload.SubjectID, AcademicYearID: payload.AcademicYearID,
		Score: payload.Score, MaxScore: payload.MaxScore, RecordedAt: recordedAt, OccurredAt: occurredAt,
	})
}

func (p *Projection) handleSumEvent(ctx context.Context, tenantID, metricName, bucketDate string, event tenancy.CloudEvent, field string) error {
	var payload map[string]any
	if len(event.Data) > 0 {
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			p.log.Warn("failed to unmarshal sum payload", "metric", metricName, "err", err)
			return nil
		}
	}
	amount, ok := extractFloat(payload, field)
	if !ok {
		p.log.Info("sum event missing amount", "metric", metricName, "field", field)
		return nil
	}
	metric, err := domain.NewMetric(tenantID, metricName, bucketDate, amount, domain.UnitSum, nil)
	if err != nil {
		return err
	}
	return p.applyMetricEvent(ctx, tenantID, event, []*domain.Metric{metric})
}

func (p *Projection) applyCountEvent(
	ctx context.Context,
	tenantID, bucketDate string,
	event tenancy.CloudEvent,
	metricName string,
	dims domain.Dimensions,
) error {
	m, err := domain.NewMetric(tenantID, metricName, bucketDate, 1, domain.UnitCount, dims)
	if err != nil {
		return err
	}
	return p.applyMetricEvent(ctx, tenantID, event, []*domain.Metric{m})
}

func (p *Projection) handleStudentCountEvent(ctx context.Context, tenantID, bucketDate string, event tenancy.CloudEvent, metricName string) error {
	var payload struct {
		StudentID string `json:"student_id"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return fmt.Errorf("%w: decode %s payload: %w", domain.ErrValidation, event.Type, err)
	}
	if strings.TrimSpace(payload.StudentID) == "" {
		return fmt.Errorf("%w: %s missing student_id", domain.ErrValidation, event.Type)
	}
	return p.applyCountEvent(ctx, tenantID, bucketDate, event, metricName, domain.Dimensions{"student_id": payload.StudentID})
}

func (p *Projection) applyMetricEvent(ctx context.Context, tenantID string, event tenancy.CloudEvent, metrics []*domain.Metric) error {
	if strings.TrimSpace(event.ID) == "" {
		return fmt.Errorf("%w: %s event missing id", domain.ErrValidation, event.Type)
	}
	return p.repo.ApplyMetricEvent(ctx, tenantID, event.ID, event.Type, metrics)
}

func extractFloat(payload map[string]any, key string) (float64, bool) {
	if payload == nil {
		return 0, false
	}
	raw, ok := payload[key]
	if !ok {
		return 0, false
	}
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		if f, err := parseNumeric(v); err == nil {
			return f, true
		}
	}
	return 0, false
}

func parseNumeric(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
