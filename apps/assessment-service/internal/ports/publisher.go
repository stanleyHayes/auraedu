// Package ports defines the outbound interfaces for the assessment-service application layer.
package ports

import (
	"context"

	"github.com/auraedu/assessment-service/internal/domain"
)

// EventPublisher emits assessment domain events.
type EventPublisher interface {
	// PublishAssessment emits an event about an Assessment aggregate.
	PublishAssessment(ctx context.Context, eventType string, assessment *domain.Assessment, meta map[string]any) error
	// PublishAssignment emits an event about an assignment (contract-conformant payload).
	PublishAssignment(ctx context.Context, eventType string, assignment *domain.Assessment, meta map[string]any) error
	// PublishScore emits an event about a Score aggregate.
	PublishScore(ctx context.Context, eventType string, score *domain.Score, meta map[string]any) error
}
