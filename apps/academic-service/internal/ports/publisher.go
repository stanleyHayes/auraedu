// Package ports defines the outbound ports for the academic-service application layer.
package ports

import (
	"context"

	"github.com/auraedu/academic-service/internal/domain"
)

// EventPublisher emits academic domain events.
type EventPublisher interface {
	// PublishYear sends an academic year domain event. meta may contain extra event data such as changed_fields.
	PublishYear(ctx context.Context, eventType string, year *domain.AcademicYear, meta map[string]any) error
	// PublishTerm sends a term domain event. meta may contain extra event data such as changed_fields.
	PublishTerm(ctx context.Context, eventType string, term *domain.Term, meta map[string]any) error
	// PublishClass sends a class domain event. meta may contain extra event data such as changed_fields.
	PublishClass(ctx context.Context, eventType string, class *domain.Class, meta map[string]any) error
	// PublishSubject sends a subject domain event. meta may contain extra event data such as changed_fields.
	PublishSubject(ctx context.Context, eventType string, subject *domain.Subject, meta map[string]any) error
}
