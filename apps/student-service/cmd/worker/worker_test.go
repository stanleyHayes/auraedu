package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/auraedu/platform/observ"
	"github.com/auraedu/student-service/internal/ports"
)

type fakeOutbox struct {
	items     []ports.OutboxEvent
	published []string
	failed    map[string]string
}

func (f *fakeOutbox) ClaimPendingStudentEvents(context.Context, int) ([]ports.OutboxEvent, error) {
	items := f.items
	f.items = nil
	return items, nil
}
func (f *fakeOutbox) MarkStudentEventPublished(_ context.Context, id string) error {
	f.published = append(f.published, id)
	return nil
}
func (f *fakeOutbox) MarkStudentEventFailed(_ context.Context, id, message string) error {
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
func metrics() *observ.WorkerMetrics {
	return observ.NewWorkerMetrics("student-worker-test", "outbox-batch", "outbox-publish")
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestDispatchPublishesAndMarks(t *testing.T) {
	payload := mustJSON(t, map[string]any{"student_id": "student-1"})
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-1", TenantID: "school-a", EventType: "student.enrolled.v1", Payload: payload}}}
	if err := dispatch(context.Background(), repo, fakePublisher{}, metrics()); err != nil {
		t.Fatal(err)
	}
	if len(repo.published) != 1 {
		t.Fatalf("published=%v failed=%v", repo.published, repo.failed)
	}
}
func TestDispatchDefersBrokerFailure(t *testing.T) {
	payload := mustJSON(t, map[string]any{"guardian_id": "guardian-1"})
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-2", Payload: payload}}}
	if err := dispatch(context.Background(), repo, fakePublisher{err: errors.New("broker down")}, metrics()); err != nil {
		t.Fatal(err)
	}
	if repo.failed["event-2"] != "broker down" {
		t.Fatalf("failed=%v", repo.failed)
	}
}
func TestDispatchRejectsMalformedPayload(t *testing.T) {
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-3", Payload: json.RawMessage(`{`)}}}
	if err := dispatch(context.Background(), repo, fakePublisher{}, metrics()); err != nil {
		t.Fatal(err)
	}
	if repo.failed["event-3"] != "invalid outbox payload" {
		t.Fatalf("failed=%v", repo.failed)
	}
}
