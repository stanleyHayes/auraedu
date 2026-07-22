package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/auraedu/platform/observ"
	"github.com/auraedu/website-service/internal/ports"
)

type fakeOutbox struct {
	items     []ports.OutboxEvent
	published []string
	failed    map[string]string
}

func (f *fakeOutbox) ClaimPendingWebsiteEvents(context.Context, int) ([]ports.OutboxEvent, error) {
	items := f.items
	f.items = nil
	return items, nil
}
func (f *fakeOutbox) MarkWebsiteEventPublished(_ context.Context, id string) error {
	f.published = append(f.published, id)
	return nil
}
func (f *fakeOutbox) MarkWebsiteEventFailed(_ context.Context, id, message string) error {
	if f.failed == nil {
		f.failed = map[string]string{}
	}
	f.failed[id] = message
	return nil
}

type fakePublisher struct{ err error }

func (f fakePublisher) PublishWithID(context.Context, string, string, string, map[string]any) error {
	return f.err
}
func testMetrics() *observ.WorkerMetrics {
	return observ.NewWorkerMetrics("website-worker-test", "outbox-batch", "outbox-publish")
}

func TestDispatchOutboxPublishesAndMarks(t *testing.T) {
	payload, err := json.Marshal(map[string]any{"page_id": "page-1"})
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-1", Payload: payload}}}
	if err := dispatchOutbox(context.Background(), repo, fakePublisher{}, testMetrics()); err != nil {
		t.Fatal(err)
	}
	if len(repo.published) != 1 {
		t.Fatalf("published=%v failed=%v", repo.published, repo.failed)
	}
}
func TestDispatchOutboxRetriesBrokerFailure(t *testing.T) {
	payload, err := json.Marshal(map[string]any{"page_id": "page-1"})
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-2", Payload: payload}}}
	if err := dispatchOutbox(context.Background(), repo, fakePublisher{err: errors.New("broker down")}, testMetrics()); err != nil {
		t.Fatal(err)
	}
	if repo.failed["event-2"] != "broker down" {
		t.Fatalf("failed=%v", repo.failed)
	}
}
func TestDispatchOutboxRejectsMalformedPayload(t *testing.T) {
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-3", Payload: json.RawMessage(`{`)}}}
	if err := dispatchOutbox(context.Background(), repo, fakePublisher{}, testMetrics()); err != nil {
		t.Fatal(err)
	}
	if repo.failed["event-3"] != "invalid outbox payload" {
		t.Fatalf("failed=%v", repo.failed)
	}
}
