package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/auraedu/platform/observ"
	"github.com/auraedu/staff-service/internal/ports"
)

type fakeOutbox struct {
	items     []ports.OutboxEvent
	published []string
	failed    map[string]string
}

func (f *fakeOutbox) ClaimPendingStaffEvents(context.Context, int) ([]ports.OutboxEvent, error) {
	items := f.items
	f.items = nil
	return items, nil
}
func (f *fakeOutbox) MarkStaffEventPublished(_ context.Context, id string) error {
	f.published = append(f.published, id)
	return nil
}
func (f *fakeOutbox) MarkStaffEventFailed(_ context.Context, id, message string) error {
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

func workerMetrics() *observ.WorkerMetrics {
	return observ.NewWorkerMetrics("staff-worker-test", "outbox-batch", "outbox-publish")
}

func mustMarshalPayload(t *testing.T, value map[string]any) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal outbox payload: %v", err)
	}
	return payload
}

func TestDispatchPublishesAndMarks(t *testing.T) {
	payload := mustMarshalPayload(t, map[string]any{"staff_id": "staff-1"})
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-1", TenantID: "school-a", EventType: "staff.created.v1", Payload: payload}}}
	if err := dispatch(context.Background(), repo, fakePublisher{}, workerMetrics()); err != nil {
		t.Fatal(err)
	}
	if len(repo.published) != 1 || repo.published[0] != "event-1" {
		t.Fatalf("published=%v failed=%v", repo.published, repo.failed)
	}
}

func TestDispatchDefersBrokerFailure(t *testing.T) {
	payload := mustMarshalPayload(t, map[string]any{"staff_id": "staff-1"})
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-2", TenantID: "school-a", EventType: "staff.updated.v1", Payload: payload}}}
	if err := dispatch(context.Background(), repo, fakePublisher{err: errors.New("broker down")}, workerMetrics()); err != nil {
		t.Fatal(err)
	}
	if repo.failed["event-2"] != "broker down" || len(repo.published) != 0 {
		t.Fatalf("published=%v failed=%v", repo.published, repo.failed)
	}
}

func TestDispatchRejectsMalformedPayload(t *testing.T) {
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-3", Payload: json.RawMessage(`{`)}}}
	if err := dispatch(context.Background(), repo, fakePublisher{}, workerMetrics()); err != nil {
		t.Fatal(err)
	}
	if repo.failed["event-3"] != "invalid outbox payload" {
		t.Fatalf("failed=%v", repo.failed)
	}
}
