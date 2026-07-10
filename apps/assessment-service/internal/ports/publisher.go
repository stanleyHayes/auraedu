package ports

import (
	"context"

	"github.com/auraedu/assessment-service/internal/domain"
)

// EventPublisher emits assessment domain events.
type EventPublisher interface {
	// PublishAssessment emits an event about an Assessment aggregate.
	PublishAssessment(ctx context.Context, eventType string, assessment *domain.Assessment, meta map[string]any) error
	// PublishScore emits an event about a Score aggregate.
	PublishScore(ctx context.Context, eventType string, score *domain.Score, meta map[string]any) error
}
