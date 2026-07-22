// Package events adapts the platform eventbus to the identity service EventPublisher port.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/identity-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
)

// Publisher adapts the platform eventbus to the identity service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher. A nil bus disables publishing.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// Publish emits the given event as a CloudEvent on the JetStream AURA stream.
func (p *Publisher) Publish(ctx context.Context, e ports.Event) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(e.Type, e.Source, e.ID, e.TenantID, e.Data)
	if err != nil {
		return fmt.Errorf("identity: build event %q: %w", e.Type, err)
	}
	// The caller-supplied event id is stable for transactional-outbox retries;
	// use it as JetStream's deduplication key as well.
	event.IdempotencyKey = e.ID
	if !e.Time.IsZero() {
		event.Time = e.Time.UTC().Format(time.RFC3339)
	}
	return p.bus.Publish(ctx, event)
}

// RecordingPublisher records events for tests.
type RecordingPublisher struct {
	Events []ports.Event
}

var _ ports.EventPublisher = (*RecordingPublisher)(nil)

// NewRecordingPublisher creates a new recording publisher.
func NewRecordingPublisher() *RecordingPublisher { return &RecordingPublisher{} }

// Publish records the event without emitting it.
func (r *RecordingPublisher) Publish(_ context.Context, e ports.Event) error {
	r.Events = append(r.Events, e)
	return nil
}
