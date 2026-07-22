package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/auraedu/academic-service/internal/ports"
	"github.com/auraedu/platform/observ"
)

type fakeOutbox struct {
	items     []ports.OutboxEvent
	published []string
	failed    map[string]string
}

func (f *fakeOutbox) ClaimPendingAcademicEvents(context.Context, int) ([]ports.OutboxEvent, error) {
	x := f.items
	f.items = nil
	return x, nil
}
func (f *fakeOutbox) MarkAcademicEventPublished(_ context.Context, id string) error {
	f.published = append(f.published, id)
	return nil
}
func (f *fakeOutbox) MarkAcademicEventFailed(_ context.Context, id, msg string) error {
	if f.failed == nil {
		f.failed = map[string]string{}
	}
	f.failed[id] = msg
	return nil
}

type fakePublisher struct{ err error }

func (f fakePublisher) PublishWithID(context.Context, string, string, string, map[string]any) error {
	return f.err
}
func metrics() *observ.WorkerMetrics {
	return observ.NewWorkerMetrics("academic-worker-test", "outbox-batch", "outbox-publish")
}

func mustPayload(t *testing.T, value map[string]any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return payload
}

func TestDispatch(t *testing.T) {
	p := mustPayload(t, map[string]any{"year_id": "1"})
	r := &fakeOutbox{items: []ports.OutboxEvent{{ID: "e", Payload: p}}}
	if err := dispatch(context.Background(), r, fakePublisher{}, metrics()); err != nil {
		t.Fatal(err)
	}
	if len(r.published) != 1 {
		t.Fatal(r.published)
	}
}
func TestDispatchFailures(t *testing.T) {
	p := mustPayload(t, map[string]any{"year_id": "1"})
	r := &fakeOutbox{items: []ports.OutboxEvent{{ID: "e", Payload: p}}}
	if err := dispatch(context.Background(), r, fakePublisher{err: errors.New("broker down")}, metrics()); err != nil {
		t.Fatalf("dispatch provider failure: %v", err)
	}
	if r.failed["e"] != "broker down" {
		t.Fatal(r.failed)
	}
	r = &fakeOutbox{items: []ports.OutboxEvent{{ID: "bad", Payload: json.RawMessage(`{`)}}}
	if err := dispatch(context.Background(), r, fakePublisher{}, metrics()); err != nil {
		t.Fatalf("dispatch malformed payload: %v", err)
	}
	if r.failed["bad"] != "invalid outbox payload" {
		t.Fatal(r.failed)
	}
}
