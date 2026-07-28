package domain

import (
	"fmt"
	"time"
)

// ListFilter narrows the audit log list query (audit.v1.yaml listAuditLogs
// query parameters). All fields are optional; the zero value matches every
// record in the actor's scope.
type ListFilter struct {
	// EventType matches the exact CloudEvents event type.
	EventType string
	// ActorID matches the actor that triggered the event. It must be a valid
	// UUID; format validation happens at the adapter boundary.
	ActorID string
	// SourceService matches the emitting service.
	SourceService string
	// From is the inclusive lower bound on the event timestamp.
	From *time.Time
	// To is the inclusive upper bound on the event timestamp.
	To *time.Time
}

// Validate checks that the filter bounds are coherent.
func (f ListFilter) Validate() error {
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return fmt.Errorf("%w: from must not be later than to", ErrValidation)
	}
	return nil
}
