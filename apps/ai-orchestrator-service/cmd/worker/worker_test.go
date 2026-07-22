package workercmd

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/auraedu/ai-orchestrator-service/internal/ports"
	"github.com/auraedu/platform/observ"
)

type fakeRepo struct {
	items []ports.OutboxEvent
	sent  []string
	bad   map[string]string
}

func (f *fakeRepo) ClaimPending(context.Context, int) ([]ports.OutboxEvent, error) {
	return f.items, nil
}
func (f *fakeRepo) MarkPublished(_ context.Context, id string) error {
	f.sent = append(f.sent, id)
	return nil
}
func (f *fakeRepo) MarkFailed(_ context.Context, id, message string) error {
	if f.bad == nil {
		f.bad = map[string]string{}
	}
	f.bad[id] = message
	return nil
}

type fakePublisher struct{ fail string }

func (f *fakePublisher) PublishWithID(_ context.Context, id, _, _ string, _ map[string]any) error {
	if id == f.fail {
		return errors.New("broker unavailable")
	}
	return nil
}

func TestDispatchRetainsMalformedAndBrokerFailures(t *testing.T) {
	repo := &fakeRepo{items: []ports.OutboxEvent{
		{ID: "invalid", Payload: []byte("{")},
		{ID: "broker", Payload: []byte(`{"message_id":"one"}`)},
		{ID: "sent", Payload: []byte(`{"message_id":"two"}`)},
	}}
	metrics := observ.NewWorkerMetrics("assistant-worker-test", "outbox-publish")
	if err := dispatch(context.Background(), repo, &fakePublisher{fail: "broker"}, slog.New(slog.NewTextHandler(io.Discard, nil)), metrics); err != nil {
		t.Fatal(err)
	}
	if repo.bad["invalid"] != "invalid outbox payload" || repo.bad["broker"] != "broker unavailable" {
		t.Fatalf("failure outcomes=%+v", repo.bad)
	}
	if len(repo.sent) != 1 || repo.sent[0] != "sent" {
		t.Fatalf("published outcomes=%+v", repo.sent)
	}
}

func TestNormalizeNATSURL(t *testing.T) {
	if got := normalizeNATSURL("nats:4222"); got != "nats://nats:4222" {
		t.Fatalf("got %q", got)
	}
}
