// Package events is a minimal local stub for platform/eventbus (AURA-2.6).
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/auraedu/identity-service/internal/ports"
	"github.com/auraedu/platform/config"
	"github.com/nats-io/nats.go"
)

type Publisher struct {
	nc     *nats.Conn
	topic  string
	logger *slog.Logger
}

func NewPublisher(ctx context.Context, logger *slog.Logger) (ports.EventPublisher, error) {
	if natsURL := config.Getenv("NATS_URL", ""); natsURL != "" {
		nc, err := nats.Connect(natsURL)
		if err != nil {
			return nil, fmt.Errorf("nats connect: %w", err)
		}
		return &Publisher{nc: nc, topic: "auraedu.events", logger: logger}, nil
	}
	return &Publisher{logger: logger}, nil
}

func (p *Publisher) Publish(ctx context.Context, e ports.Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if p.nc != nil {
		return p.nc.Publish(p.topic, body)
	}
	if p.logger != nil {
		p.logger.Info("event published", "type", e.Type, "tenant_id", e.TenantID, "id", e.ID)
	}
	return nil
}

type RecordingPublisher struct {
	Events []ports.Event
}

func NewRecordingPublisher() *RecordingPublisher { return &RecordingPublisher{} }

func (r *RecordingPublisher) Publish(ctx context.Context, e ports.Event) error {
	r.Events = append(r.Events, e)
	return nil
}
