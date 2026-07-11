// Package events is a minimal local stub for platform/eventbus (AURA-2.6).
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/tenant-service/internal/ports"
	"github.com/nats-io/nats.go"
)

type Publisher struct {
	nc     *nats.Conn
	topic  string
	logger *slog.Logger
}

func NewPublisher(_ context.Context, logger *slog.Logger) (ports.EventPublisher, error) {
	if natsURL := config.Getenv("NATS_URL", ""); natsURL != "" {
		nc, err := nats.Connect(natsURL)
		if err != nil {
			return nil, fmt.Errorf("nats connect: %w", err)
		}
		return &Publisher{nc: nc, topic: "auraedu.events", logger: logger}, nil
	}
	return &Publisher{logger: logger}, nil
}

func (p *Publisher) Publish(_ context.Context, eventType, tenantCode string, payload map[string]any) error {
	body, err := json.Marshal(map[string]any{
		"specversion": "1.0",
		"type":        eventType,
		"source":      "tenant-service",
		"tenant_id":   tenantCode,
		"data":        payload,
	})
	if err != nil {
		return err
	}
	if p.nc != nil {
		return p.nc.Publish(p.topic, body)
	}
	if p.logger != nil {
		p.logger.Info("event published", "type", eventType, "tenant_code", tenantCode)
	}
	return nil
}

type RecordingPublisher struct {
	Events []struct {
		Type       string
		TenantCode string
		Payload    map[string]any
	}
}

func NewRecordingPublisher() *RecordingPublisher { return &RecordingPublisher{} }

func (r *RecordingPublisher) Publish(_ context.Context, eventType, tenantCode string, payload map[string]any) error {
	r.Events = append(r.Events, struct {
		Type       string
		TenantCode string
		Payload    map[string]any
	}{eventType, tenantCode, payload})
	return nil
}
