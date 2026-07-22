// Package ports defines the outbound interfaces for the analytics-service application layer.
package ports

import (
	"context"

	"github.com/auraedu/analytics-service/internal/domain"
)

// Repository persists Metric aggregates. Implementations MUST scope every
// query by tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	UpsertMetric(ctx context.Context, tenantID string, m *domain.Metric) error
	ApplyMetricEvent(ctx context.Context, tenantID, eventID, eventType string, metrics []*domain.Metric) error
	ApplyAssessmentScoreEvent(ctx context.Context, tenantID string, event domain.AssessmentScoreEvent) error
	ListMetrics(ctx context.Context, tenantID string, filter ListFilter) ([]*domain.Metric, string, error)
	ApplyGrowthEvent(ctx context.Context, tenantID string, event domain.GrowthEvent) error
	GrowthRollups(ctx context.Context, tenantID, fromDate, toDate string) ([]domain.GrowthRollup, error)
}

// LearnerScope is the authoritative set of learner records an actor may see.
type LearnerScope struct {
	StudentIDs []string
}

// LearnerScopeResolver resolves role-scoped learner access without exposing
// Student Service storage to Analytics.
type LearnerScopeResolver interface {
	Resolve(ctx context.Context, tenantID, userID, role string) (LearnerScope, error)
}

// ListFilter carries cursor pagination and optional filters for listing metrics.
type ListFilter struct {
	Limit          int
	Cursor         string
	MetricName     string
	BucketDateFrom string
	BucketDateTo   string
	DimensionKey   string
	DimensionValue string
	// StudentIDs is server-owned authorization scope and is never populated
	// directly from a public query parameter.
	StudentIDs []string
}
