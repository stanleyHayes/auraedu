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
	case "student.enrolled.v1":
		return p.incrementCount(ctx, tenantID, "students.count", bucketDate, nil)

	case "attendance.marked.v1":
		return p.handleAttendanceMarked(ctx, tenantID, bucketDate, event.Data)

	case "assessment.score_recorded.v1":
		return p.handleAssessmentScore(ctx, tenantID, bucketDate, event.Data)

	case "payment.received.v1":
		return p.handleSumEvent(ctx, tenantID, "payments.total", bucketDate, event.Data, "amount")

	case "invoice.created.v1":
		return p.handleSumEvent(ctx, tenantID, "invoices.total", bucketDate, event.Data, "amount")

	case "report.published.v1":
		return p.incrementCount(ctx, tenantID, "reports.count", bucketDate, nil)

	default:
		p.log.Info("ignoring unsupported event type", "type", event.Type, "id", event.ID)
		return nil
	}
}

func (p *Projection) handleAttendanceMarked(ctx context.Context, tenantID, bucketDate string, data json.RawMessage) error {
	var payload struct {
		Status string `json:"status"`
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			p.log.Warn("attendance.marked.v1: failed to unmarshal payload", "err", err)
		}
	}
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	switch status {
	case "present", "absent", "late", "excused":
		name := fmt.Sprintf("attendance.%s", status)
		return p.incrementCount(ctx, tenantID, name, bucketDate, nil)
	default:
		p.log.Info("attendance.marked.v1: unknown status", "status", payload.Status)
		return nil
	}
}

func (p *Projection) handleAssessmentScore(ctx context.Context, tenantID, bucketDate string, data json.RawMessage) error {
	var payload struct {
		Score          float64 `json:"score"`
		SubjectID      string  `json:"subject_id"`
		AcademicYearID string  `json:"academic_year_id"`
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			p.log.Warn("assessment.score_recorded.v1: failed to unmarshal payload", "err", err)
			return nil
		}
	}
	if payload.SubjectID == "" || payload.AcademicYearID == "" {
		p.log.Info("assessment.score_recorded.v1: missing subject_id or academic_year_id")
		return nil
	}

	dims := domain.Dimensions{
		"subject_id":       payload.SubjectID,
		"academic_year_id": payload.AcademicYearID,
	}

	if err := p.incrementCount(ctx, tenantID, "assessments.count", bucketDate, dims); err != nil {
		return err
	}
	if err := p.addToSum(ctx, tenantID, "assessments.sum_score", bucketDate, dims, payload.Score); err != nil {
		return err
	}
	return p.addSampleToAverage(ctx, tenantID, "assessments.avg_score", bucketDate, dims, payload.Score)
}

func (p *Projection) handleSumEvent(ctx context.Context, tenantID, metricName, bucketDate string, data json.RawMessage, field string) error {
	var payload map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			p.log.Warn("failed to unmarshal sum payload", "metric", metricName, "err", err)
			return nil
		}
	}
	amount, ok := extractFloat(payload, field)
	if !ok {
		p.log.Info("sum event missing amount", "metric", metricName, "field", field)
		return nil
	}
	return p.addToSum(ctx, tenantID, metricName, bucketDate, nil, amount)
}

func (p *Projection) incrementCount(ctx context.Context, tenantID, metricName, bucketDate string, dims domain.Dimensions) error {
	m, err := domain.NewMetric(tenantID, metricName, bucketDate, 1, domain.UnitCount, dims)
	if err != nil {
		return err
	}
	return p.repo.UpsertMetric(ctx, tenantID, m)
}

func (p *Projection) addToSum(ctx context.Context, tenantID, metricName, bucketDate string, dims domain.Dimensions, amount float64) error {
	m, err := domain.NewMetric(tenantID, metricName, bucketDate, amount, domain.UnitSum, dims)
	if err != nil {
		return err
	}
	return p.repo.UpsertMetric(ctx, tenantID, m)
}

func (p *Projection) addSampleToAverage(ctx context.Context, tenantID, metricName, bucketDate string, dims domain.Dimensions, sample float64) error {
	m, err := domain.NewMetric(tenantID, metricName, bucketDate, sample, domain.UnitAverage, dims)
	if err != nil {
		return err
	}
	return p.repo.UpsertMetric(ctx, tenantID, m)
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
