package tenancy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type CloudEvent struct {
	SpecVersion string `json:"specversion"`
	Type        string `json:"type"`
	Source      string `json:"source"`
	Subject     string `json:"subject,omitempty"`
	ID          string `json:"id"`
	Time        string `json:"time,omitempty"`
	TenantID    string `json:"tenant_id"`
	// IdempotencyKey is transport metadata used to populate Nats-Msg-Id. It is
	// deliberately excluded from the contracted CloudEvent JSON envelope.
	IdempotencyKey string          `json:"-"`
	Data           json.RawMessage `json:"data,omitempty"`
}

var (
	ErrMissingEventTenant   = errors.New("tenancy: CloudEvent missing tenant_id")
	ErrMissingEventID       = errors.New("tenancy: CloudEvent missing id")
	ErrMissingEventType     = errors.New("tenancy: CloudEvent missing type")
	ErrUnversionedEventType = errors.New("tenancy: CloudEvent type must be versioned")
	ErrMissingEventSource   = errors.New("tenancy: CloudEvent missing source")
	ErrMissingEventTime     = errors.New("tenancy: CloudEvent missing time")
	ErrInvalidEventTime     = errors.New("tenancy: CloudEvent time must be RFC3339")
	ErrMissingEventData     = errors.New("tenancy: CloudEvent missing data")
	ErrInvalidEventData     = errors.New("tenancy: CloudEvent data must be a JSON object")
)

var versionedEventType = regexp.MustCompile(`^[a-z][a-z0-9_-]*(?:\.[a-z0-9_-]+)+\.v[1-9][0-9]*$`)

func (e CloudEvent) Validate() error {
	if e.SpecVersion != "1.0" {
		return fmt.Errorf("tenancy: unsupported CloudEvents specversion %q", e.SpecVersion)
	}
	if strings.TrimSpace(e.Type) == "" {
		return ErrMissingEventType
	}
	if !versionedEventType.MatchString(e.Type) {
		return fmt.Errorf("%w: %q", ErrUnversionedEventType, e.Type)
	}
	if strings.TrimSpace(e.Source) == "" {
		return ErrMissingEventSource
	}
	if e.ID == "" {
		return ErrMissingEventID
	}
	if strings.TrimSpace(e.Time) == "" {
		return ErrMissingEventTime
	}
	if _, err := time.Parse(time.RFC3339, e.Time); err != nil {
		return ErrInvalidEventTime
	}
	if e.TenantID == "" {
		return ErrMissingEventTenant
	}
	data := bytes.TrimSpace(e.Data)
	if len(data) == 0 {
		return ErrMissingEventData
	}
	if !json.Valid(data) || data[0] != '{' {
		return ErrInvalidEventData
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
		Time:        time.Now().UTC().Format(time.RFC3339),
		TenantID:    tenantID,
		Data:        raw,
	}, nil
}
