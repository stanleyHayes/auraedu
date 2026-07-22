package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/auraedu/cbt-service/internal/ports"
	"github.com/auraedu/platform/observ"
)

type fakeOutbox struct {
	items     []ports.OutboxEvent
	published []string
	failed    map[string]string
}

func (f *fakeOutbox) ClaimPendingCBTEvents(context.Context, int) ([]ports.OutboxEvent, error) {
	x := f.items
	f.items = nil
	return x, nil
}
func (f *fakeOutbox) MarkCBTEventPublished(_ context.Context, id string) error {
	f.published = append(f.published, id)
	return nil
}
func (f *fakeOutbox) MarkCBTEventFailed(_ context.Context, id, msg string) error {
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
func wm() *observ.WorkerMetrics {
	return observ.NewWorkerMetrics("cbt-worker-test", "outbox-batch", "outbox-publish")
}
func TestDispatch(t *testing.T) {
	p, err := json.Marshal(map[string]any{"exam_id": "1"})
	if err != nil {
		t.Fatal(err)
	}
	r := &fakeOutbox{items: []ports.OutboxEvent{{ID: "e", Payload: p}}}
	if err := dispatch(context.Background(), r, fakePublisher{}, wm()); err != nil {
		t.Fatal(err)
	}
	if len(r.published) != 1 {
		t.Fatal(r.published)
	}
}
func TestDispatchFailures(t *testing.T) {
	p, err := json.Marshal(map[string]any{"exam_id": "1"})
	if err != nil {
		t.Fatal(err)
	}
	r := &fakeOutbox{items: []ports.OutboxEvent{{ID: "e", Payload: p}}}
	if err := dispatch(context.Background(), r, fakePublisher{errors.New("broker down")}, wm()); err != nil {
		t.Fatal(err)
	}
	if r.failed["e"] != "broker down" {
		t.Fatal(r.failed)
	}
	r = &fakeOutbox{items: []ports.OutboxEvent{{ID: "bad", Payload: json.RawMessage(`{`)}}}
	if err := dispatch(context.Background(), r, fakePublisher{}, wm()); err != nil {
		t.Fatal(err)
	}
	if r.failed["bad"] != "invalid outbox payload" {
		t.Fatal(r.failed)
	}
}
