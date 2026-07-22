package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/auraedu/assessment-service/internal/ports"
	"github.com/auraedu/platform/observ"
)

type fakeOutbox struct {
	items     []ports.OutboxEvent
	published []string
	failed    map[string]string
}

func (f *fakeOutbox) ClaimPendingAssessmentEvents(context.Context, int) ([]ports.OutboxEvent, error) {
	x := f.items
	f.items = nil
	return x, nil
}
func (f *fakeOutbox) MarkAssessmentEventPublished(_ context.Context, id string) error {
	f.published = append(f.published, id)
	return nil
}
func (f *fakeOutbox) MarkAssessmentEventFailed(_ context.Context, id, msg string) error {
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
	return observ.NewWorkerMetrics("assessment-worker-test", "outbox-batch", "outbox-publish")
}
func TestDispatch(t *testing.T) {
	p, err := json.Marshal(map[string]any{"assessment_id": "1"})
	if err != nil {
		t.Fatal(err)
	}
	r := &fakeOutbox{items: []ports.OutboxEvent{{ID: "e", Payload: p}}}
	if err := dispatch(context.Background(), r, fakePublisher{}, metrics()); err != nil {
		t.Fatal(err)
	}
	if len(r.published) != 1 {
		t.Fatal(r.published)
	}
}
func TestDispatchFailures(t *testing.T) {
	p, err := json.Marshal(map[string]any{"score_id": "1"})
	if err != nil {
		t.Fatal(err)
	}
	r := &fakeOutbox{items: []ports.OutboxEvent{{ID: "e", Payload: p}}}
	if err := dispatch(context.Background(), r, fakePublisher{err: errors.New("broker down")}, metrics()); err != nil {
		t.Fatal(err)
	}
	if r.failed["e"] != "broker down" {
		t.Fatal(r.failed)
	}
	r = &fakeOutbox{items: []ports.OutboxEvent{{ID: "bad", Payload: json.RawMessage(`{`)}}}
	if err := dispatch(context.Background(), r, fakePublisher{}, metrics()); err != nil {
		t.Fatal(err)
	}
	if r.failed["bad"] != "invalid outbox payload" {
		t.Fatal(r.failed)
	}
}
