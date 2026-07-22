package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/auraedu/audit-service/internal/adapters/events"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	"github.com/nats-io/nats.go"
)

// The audit wildcard consumer overlaps every domain-specific consumer. This is
// valid only on the canonical limits-retention pub/sub stream.
func TestAuditWildcardCanCoexistWithProjectionConsumers(t *testing.T) {
	url := os.Getenv("NATS_URL")
	if url == "" {
		t.Skip("NATS_URL is not configured")
	}
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect NATS: %v", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("JetStream: %v", err)
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		t.Fatalf("ensure stream: %v", err)
	}
	received := make(chan tenancy.CloudEvent, 1)
	subscriber := events.NewSubscriber(js, nil)
	if err := subscriber.Start(context.Background(), func(_ context.Context, event tenancy.CloudEvent) error {
		received <- event
		return nil
	}); err != nil {
		t.Fatalf("register audit wildcard consumer: %v", err)
	}
	event, err := tenancy.NewCloudEvent("lead.created.v1", "crm-service", "cccccccc-cccc-cccc-cccc-cccccccccccc", "upshs", map[string]any{"lead_id": "cccccccc-cccc-cccc-cccc-cccccccccccc"})
	if err != nil {
		t.Fatalf("build event: %v", err)
	}
	if err := eventbus.NewPublisher(js).Publish(context.Background(), event); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	deadline := time.After(3 * time.Second)
	for {
		select {
		case got := <-received:
			// A durable may first drain an event published by another broker test.
			if got.ID == event.ID {
				if got.Type != "lead.created.v1" || got.TenantID != "upshs" {
					t.Fatalf("unexpected audit event: %+v", got)
				}
				goto receivedExpected
			}
		case <-deadline:
			t.Fatal("audit wildcard did not receive lead.created")
		}
	}

receivedExpected:
	if err := subscriber.Stop(); err != nil {
		t.Fatalf("stop audit consumer: %v", err)
	}
}
