package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// WebhookEvent is the aggregate root for a payment-provider webhook payload.
type WebhookEvent struct {
	ID          string          `json:"id"`
	TenantID    string          `json:"tenant_id,omitempty"`
	Provider    string          `json:"provider"`
	EventType   string          `json:"event_type"`
	Payload     json.RawMessage `json:"payload"`
	Signature   *string         `json:"signature,omitempty"`
	Processed   bool            `json:"processed"`
	ProcessedAt *time.Time      `json:"processed_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// NewWebhookEvent constructs a WebhookEvent, enforcing invariants.
func NewWebhookEvent(provider, eventType string, payload json.RawMessage, signature *string) (*WebhookEvent, error) {
	if strings.TrimSpace(provider) == "" {
		return nil, fmt.Errorf("%w: provider is required", ErrValidation)
	}
	if strings.TrimSpace(eventType) == "" {
		return nil, fmt.Errorf("%w: event_type is required", ErrValidation)
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("%w: payload is required", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("payments: generate id: %w", err)
	}
	now := time.Now().UTC()
	return &WebhookEvent{
		ID:        id.String(),
		Provider:  strings.TrimSpace(strings.ToLower(provider)),
		EventType: strings.TrimSpace(eventType),
		Payload:   payload,
		Signature: signature,
		Processed: false,
		CreatedAt: now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (w WebhookEvent) Validate() error {
	if strings.TrimSpace(w.Provider) == "" {
		return fmt.Errorf("%w: provider is required", ErrValidation)
	}
	if strings.TrimSpace(w.EventType) == "" {
		return fmt.Errorf("%w: event_type is required", ErrValidation)
	}
	if len(w.Payload) == 0 {
		return fmt.Errorf("%w: payload is required", ErrValidation)
	}
	return nil
}

// SetTenant resolves and stores the tenant_id from a JSON payload field.
func (w *WebhookEvent) SetTenant(tenantID string) {
	w.TenantID = strings.TrimSpace(tenantID)
}

// MarkProcessed records that the webhook has been handled.
func (w *WebhookEvent) MarkProcessed() {
	now := time.Now().UTC()
	w.Processed = true
	w.ProcessedAt = &now
}
