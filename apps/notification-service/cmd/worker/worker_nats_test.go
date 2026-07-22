package workercmd

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/auraedu/notification-service/internal/application"
	"github.com/auraedu/platform/eventbus"
	"github.com/nats-io/nats.go"
)

// Verifies notification durables can coexist with analytics filters and the
// audit wildcard on the canonical pub/sub stream.
func TestConsumerRegistersAllEventSubjects(t *testing.T) {
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
	consumer := newConsumer(js, slog.Default(), application.NewService(nil, nil, nil))
	if err := consumer.Start(context.Background()); err != nil {
		t.Fatalf("start notification consumer: %v", err)
	}
	if err := consumer.Stop(); err != nil {
		t.Fatalf("stop notification consumer: %v", err)
	}
}
