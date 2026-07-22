// Package domain contains the audit log aggregate and builder.
package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AuditLog is the immutable aggregate root of the audit service. Each record
// captures a single CloudEvent that crossed the platform event bus, scoped to
// exactly one tenant.
type AuditLog struct {
	ID            uuid.UUID       `json:"id"`
	TenantID      string          `json:"tenant_id"`
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	SourceService string          `json:"source_service"`
	Timestamp     time.Time       `json:"timestamp"`
	ReceivedAt    time.Time       `json:"received_at"`
	Payload       json.RawMessage `json:"payload"`
	ActorID       string          `json:"actor_id"`
	Action        string          `json:"action"`
	ResourceType  string          `json:"resource_type"`
	ResourceID    string          `json:"resource_id"`
}

// Validate checks that the audit log aggregate satisfies its invariants.
func (a AuditLog) Validate() error {
	if a.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrValidation)
	}
	if strings.TrimSpace(a.TenantID) == "" {
		return fmt.Errorf("%w: tenant_id is required", ErrValidation)
	}
	if strings.TrimSpace(a.EventID) == "" {
		return fmt.Errorf("%w: event_id is required", ErrValidation)
	}
	if strings.TrimSpace(a.EventType) == "" {
		return fmt.Errorf("%w: event_type is required", ErrValidation)
	}
	if strings.TrimSpace(a.SourceService) == "" {
		return fmt.Errorf("%w: source_service is required", ErrValidation)
	}
	if a.Timestamp.IsZero() {
		return fmt.Errorf("%w: timestamp is required", ErrValidation)
	}
	if a.ReceivedAt.IsZero() {
		return fmt.Errorf("%w: received_at is required", ErrValidation)
	}
	if strings.TrimSpace(a.Action) == "" {
		return fmt.Errorf("%w: action is required", ErrValidation)
	}
	return nil
}

// AuditLogBuilder constructs a validated AuditLog aggregate.
type AuditLogBuilder struct {
	log AuditLog
}

// NewAuditLogBuilder starts a new builder. An id is generated automatically on Build if one is not set.
func NewAuditLogBuilder() *AuditLogBuilder {
	return &AuditLogBuilder{}
}

// ID sets the audit log id. If not called, a UUID v7 is generated automatically.
func (b *AuditLogBuilder) ID(id uuid.UUID) *AuditLogBuilder {
	b.log.ID = id
	return b
}

// TenantID sets the tenant scope.
func (b *AuditLogBuilder) TenantID(id string) *AuditLogBuilder {
	b.log.TenantID = id
	return b
}

// EventID sets the original CloudEvents id.
func (b *AuditLogBuilder) EventID(id string) *AuditLogBuilder {
	b.log.EventID = id
	return b
}

// EventType sets the original CloudEvents type.
func (b *AuditLogBuilder) EventType(t string) *AuditLogBuilder {
	b.log.EventType = t
	return b
}

// SourceService sets the CloudEvents source service.
func (b *AuditLogBuilder) SourceService(s string) *AuditLogBuilder {
	b.log.SourceService = s
	return b
}

// Timestamp sets the time the event occurred.
func (b *AuditLogBuilder) Timestamp(t time.Time) *AuditLogBuilder {
	b.log.Timestamp = t
	return b
}

// ReceivedAt sets the time the event was received by the audit service.
func (b *AuditLogBuilder) ReceivedAt(t time.Time) *AuditLogBuilder {
	b.log.ReceivedAt = t
	return b
}

// Payload sets the raw JSON event payload.
func (b *AuditLogBuilder) Payload(p json.RawMessage) *AuditLogBuilder {
	b.log.Payload = p
	return b
}

// ActorID sets the actor that triggered the event.
func (b *AuditLogBuilder) ActorID(a string) *AuditLogBuilder {
	b.log.ActorID = a
	return b
}

// Action sets the action derived from the CloudEvents type.
func (b *AuditLogBuilder) Action(a string) *AuditLogBuilder {
	b.log.Action = a
	return b
}

// ResourceType sets the type of resource affected by the event.
func (b *AuditLogBuilder) ResourceType(r string) *AuditLogBuilder {
	b.log.ResourceType = r
	return b
}

// ResourceID sets the identifier of the resource affected by the event.
func (b *AuditLogBuilder) ResourceID(r string) *AuditLogBuilder {
	b.log.ResourceID = r
	return b
}

// Build returns the validated AuditLog, generating an id if necessary.
func (b *AuditLogBuilder) Build() (*AuditLog, error) {
	if b.log.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("audit: generate id: %w", err)
		}
		b.log.ID = id
	}
	if err := b.log.Validate(); err != nil {
		return nil, err
	}
	return &b.log, nil
}
