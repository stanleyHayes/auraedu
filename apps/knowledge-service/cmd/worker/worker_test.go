package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/auraedu/knowledge-service/internal/ports"
	"github.com/auraedu/platform/observ"
)

type fakeOutboxRepo struct {
	items     []ports.OutboxEvent
	published []string
	failed    map[string]string
}

func (f *fakeOutboxRepo) ClaimPending(context.Context, int) ([]ports.OutboxEvent, error) {
	return f.items, nil
}
func (f *fakeOutboxRepo) MarkPublished(_ context.Context, id string) error {
	f.published = append(f.published, id)
	return nil
}
func (f *fakeOutboxRepo) MarkFailed(_ context.Context, id, message string) error {
	if f.failed == nil {
		f.failed = map[string]string{}
	}
	f.failed[id] = message
	return nil
}

type fakeOutboxPublisher struct {
	ids    []string
	failID string
}

func (f *fakeOutboxPublisher) PublishWithID(_ context.Context, id, _, _ string, _ map[string]any) error {
	f.ids = append(f.ids, id)
	if id == f.failID {
		return errors.New("broker unavailable")
	}
	return nil
}

func TestDispatchRecordsMalformedBrokerFailureAndSuccess(t *testing.T) {
	payload, err := json.Marshal(map[string]any{"source_id": "source-1"})
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeOutboxRepo{items: []ports.OutboxEvent{
		{ID: "malformed", TenantID: "school-one", EventType: "knowledge.source_approved.v1", Payload: []byte("{")},
		{ID: "broker-failure", TenantID: "school-one", EventType: "knowledge.source_approved.v1", Payload: payload},
		{ID: "published", TenantID: "school-one", EventType: "knowledge.source_approved.v1", Payload: payload},
	}}
	pub := &fakeOutboxPublisher{failID: "broker-failure"}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	metrics := observ.NewWorkerMetrics("knowledge-worker-test", "outbox-publish")

	if err := dispatch(context.Background(), repo, pub, log, metrics); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if repo.failed["malformed"] != "invalid outbox payload" {
		t.Fatalf("malformed outcome not retained: %+v", repo.failed)
	}
	if repo.failed["broker-failure"] != "broker unavailable" {
		t.Fatalf("broker outcome not retained: %+v", repo.failed)
	}
	if len(repo.published) != 1 || repo.published[0] != "published" {
		t.Fatalf("success not marked exactly once: %+v", repo.published)
	}
	if len(pub.ids) != 2 || pub.ids[0] != "broker-failure" || pub.ids[1] != "published" {
		t.Fatalf("stable event identities not forwarded: %+v", pub.ids)
	}
}

func TestNormalizeNATSURLSupportsRenderHostport(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{input: "nats:4222", want: "nats://nats:4222"},
		{input: " nats://nats:4222 ", want: "nats://nats:4222"},
		{input: "", want: ""},
	} {
		if got := normalizeNATSURL(test.input); got != test.want {
			t.Fatalf("normalizeNATSURL(%q)=%q want %q", test.input, got, test.want)
		}
	}
}
