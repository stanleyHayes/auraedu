package ports

import (
	"context"

	"github.com/auraedu/analytics-service/internal/domain"
)

// Repository persists Metric aggregates. Implementations MUST scope every
// query by tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	UpsertMetric(ctx context.Context, tenantID string, m *domain.Metric) error
	ListMetrics(ctx context.Context, tenantID string, filter ListFilter) ([]*domain.Metric, string, error)
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
}
