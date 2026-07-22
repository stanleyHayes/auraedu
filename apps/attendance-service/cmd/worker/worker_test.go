package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/auraedu/attendance-service/internal/ports"
	"github.com/auraedu/platform/observ"
)

type fakeOutbox struct {
	items     []ports.OutboxEvent
	published []string
	failed    map[string]string
}

func (f *fakeOutbox) ClaimPendingAttendanceEvents(context.Context, int) ([]ports.OutboxEvent, error) {
	items := f.items
	f.items = nil
	return items, nil
}

func (f *fakeOutbox) MarkAttendanceEventPublished(_ context.Context, id string) error {
	f.published = append(f.published, id)
	return nil
}

func (f *fakeOutbox) MarkAttendanceEventFailed(_ context.Context, id, message string) error {
	if f.failed == nil {
		f.failed = make(map[string]string)
	}
	f.failed[id] = message
	return nil
}

type fakePublisher struct{ err error }

func (f fakePublisher) PublishWithID(context.Context, string, string, string, map[string]any) error {
	return f.err
}

func testMetrics() *observ.WorkerMetrics {
	return observ.NewWorkerMetrics("attendance-worker-test", "outbox-batch", "outbox-publish")
}

func TestDispatchPublishesAndAcknowledges(t *testing.T) {
	payload, err := json.Marshal(map[string]any{"attendance_id": "attendance-1"})
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-1", Payload: payload}}}
	if err := dispatch(context.Background(), repo, fakePublisher{}, testMetrics()); err != nil {
		t.Fatal(err)
	}
	if len(repo.published) != 1 || repo.published[0] != "event-1" {
		t.Fatalf("published events = %v", repo.published)
	}
}

func TestDispatchRecordsRetryableFailures(t *testing.T) {
	payload, err := json.Marshal(map[string]any{"attendance_id": "attendance-1"})
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-1", Payload: payload}}}
	if err := dispatch(context.Background(), repo, fakePublisher{err: errors.New("broker down")}, testMetrics()); err != nil {
		t.Fatal(err)
	}
	if repo.failed["event-1"] != "broker down" {
		t.Fatalf("failed events = %v", repo.failed)
	}

	repo = &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-bad", Payload: json.RawMessage(`{`)}}}
	if err := dispatch(context.Background(), repo, fakePublisher{}, testMetrics()); err != nil {
		t.Fatal(err)
	}
	if repo.failed["event-bad"] != "invalid outbox payload" {
		t.Fatalf("failed events = %v", repo.failed)
	}
}
