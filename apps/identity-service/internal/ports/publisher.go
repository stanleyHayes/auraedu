// Package ports defines the identity-service boundary interfaces.
package ports

import (
	"context"
	"time"
)

// Event is a CloudEvents-shaped message emitted by the Identity Service.
type Event struct {
	SpecVersion     string         `json:"specversion"`
	Type            string         `json:"type"`
	Source          string         `json:"source"`
	ID              string         `json:"id"`
	Time            time.Time      `json:"time"`
	TenantID        string         `json:"tenant_id"`
	DataContentType string         `json:"datacontenttype"`
	Data            map[string]any `json:"data"`
}

// EventPublisher emits domain events.
type EventPublisher interface {
	Publish(ctx context.Context, e Event) error
}
