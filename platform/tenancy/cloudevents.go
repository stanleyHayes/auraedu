package tenancy

import (
	"encoding/json"
	"errors"
	"fmt"
)

type CloudEvent struct {
	SpecVersion    string          `json:"specversion"`
	Type           string          `json:"type"`
	Source         string          `json:"source"`
	Subject        string          `json:"subject,omitempty"`
	ID             string          `json:"id"`
	Time           string          `json:"time,omitempty"`
	TenantID       string          `json:"tenant_id"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	Data           json.RawMessage `json:"data,omitempty"`
}

var (
	ErrMissingEventTenant = errors.New("tenancy: CloudEvent missing tenant_id")
	ErrMissingEventID     = errors.New("tenancy: CloudEvent missing id")
	ErrMissingEventType   = errors.New("tenancy: CloudEvent missing type")
)

func (e CloudEvent) Validate() error {
	if e.SpecVersion != "1.0" {
		return fmt.Errorf("tenancy: unsupported CloudEvents specversion %q", e.SpecVersion)
	}
	if e.Type == "" {
		return ErrMissingEventType
	}
	if e.ID == "" {
		return ErrMissingEventID
	}
	if e.TenantID == "" {
		return ErrMissingEventTenant
	}
	return nil
}

func (e CloudEvent) TenantContext() TenantContext {
	return TenantContext{TenantID: e.TenantID}
}

func (e CloudEvent) WithTenant(tenantID string) CloudEvent {
	e.TenantID = tenantID
	return e
}

func (e CloudEvent) WithIdempotencyKey(key string) CloudEvent {
	e.IdempotencyKey = key
	return e
}

func NewCloudEvent(eventType, source, id, tenantID string, data any) (CloudEvent, error) {
	var raw json.RawMessage
	if data != nil {
		var err error
		raw, err = json.Marshal(data)
		if err != nil {
			return CloudEvent{}, fmt.Errorf("tenancy: marshal event data: %w", err)
		}
	}
	return CloudEvent{
		SpecVersion: "1.0",
		Type:        eventType,
		Source:      source,
		ID:          id,
		TenantID:    tenantID,
		Data:        raw,
	}, nil
}
