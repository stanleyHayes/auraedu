// Package ports defines the report service publisher boundary.
package ports

import (
	"context"

	"github.com/auraedu/report-service/internal/domain"
)

// EventPublisher emits report domain events.
type EventPublisher interface {
	// PublishReportTemplate emits an event about a ReportTemplate aggregate.
	PublishReportTemplate(ctx context.Context, eventType string, t *domain.ReportTemplate, meta map[string]any) error
	// PublishReportCard emits an event about a ReportCard aggregate.
	PublishReportCard(ctx context.Context, eventType string, c *domain.ReportCard, meta map[string]any) error
}
