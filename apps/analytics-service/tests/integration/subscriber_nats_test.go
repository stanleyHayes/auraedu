package integration

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/auraedu/analytics-service/internal/adapters/events"
	"github.com/auraedu/analytics-service/internal/application"
	"github.com/auraedu/analytics-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	"github.com/nats-io/nats.go"
)

// TestSubscriberRegistersEveryFilteredDurable catches durable-name/filter
// collisions that unit projection tests cannot see. Set NATS_URL in CI/local
// smoke jobs; ordinary package tests skip when no broker is available.
func TestSubscriberRegistersEveryFilteredDurable(t *testing.T) {
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
	repo := newMemoryRepo()
	subscriber := events.NewSubscriber(js, application.NewProjection(repo, slog.Default()), slog.Default())
	if err := subscriber.Start(context.Background()); err != nil {
		t.Fatalf("register analytics consumers: %v", err)
	}
	event, err := tenancy.NewCloudEvent("lead.created.v1", "crm-service", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", tenantA, map[string]any{"lead_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "stage": "new", "source": "website", "created_at": time.Now().UTC()})
	if err != nil {
		t.Fatalf("build lead event: %v", err)
	}
	if err := eventbus.NewPublisher(js).Publish(context.Background(), event); err != nil {
		t.Fatalf("publish lead event: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		metrics, _, listErr := repo.ListMetrics(context.Background(), tenantA, ports.ListFilter{MetricName: "growth.leads.count", Limit: 10})
		if listErr != nil {
			t.Fatalf("list projected metrics: %v", listErr)
		}
		if len(metrics) == 1 && metrics[0].Value == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("lead event was not projected: %+v", metrics)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := subscriber.Stop(); err != nil {
		t.Fatalf("stop analytics consumers: %v", err)
	}
}
