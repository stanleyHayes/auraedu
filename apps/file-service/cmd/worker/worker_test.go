package workercmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/auraedu/file-service/internal/ports"
	"github.com/auraedu/platform/observ"
)

type fakeOutbox struct {
	items     []ports.OutboxEvent
	published []string
	failed    map[string]string
}

func (f *fakeOutbox) ClaimPendingFileEvents(context.Context, int) ([]ports.OutboxEvent, error) {
	items := f.items
	f.items = nil
	return items, nil
}
func (f *fakeOutbox) MarkFileEventPublished(_ context.Context, id string) error {
	f.published = append(f.published, id)
	return nil
}
func (f *fakeOutbox) MarkFileEventFailed(_ context.Context, id, message string) error {
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

type fakeStorage struct {
	deleted []string
	err     error
}

func (f *fakeStorage) Backend() string { return "local" }
func (f *fakeStorage) Save(context.Context, string, string, io.Reader) (string, error) {
	return "", nil
}
func (f *fakeStorage) Open(context.Context, string, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}
func (f *fakeStorage) Delete(_ context.Context, tenant, path string) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, tenant+":"+path)
	return nil
}

func metrics() *observ.WorkerMetrics {
	return observ.NewWorkerMetrics("file-worker-test", "outbox-batch", "outbox-publish")
}
func logger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestDispatchCleansStoragePublishesAndMarks(t *testing.T) {
	payload := mustJSON(t, map[string]any{"file_id": "file-1"})
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-1", TenantID: "school-a", EventType: "file.deleted.v1", Payload: payload, CleanupPath: "school-a/file-1"}}}
	store := &fakeStorage{}
	if err := dispatch(context.Background(), repo, store, fakePublisher{}, logger(), metrics()); err != nil {
		t.Fatal(err)
	}
	if len(store.deleted) != 1 || len(repo.published) != 1 {
		t.Fatalf("deleted=%v published=%v failed=%v", store.deleted, repo.published, repo.failed)
	}
}

func TestDispatchDefersCleanupOrBrokerFailure(t *testing.T) {
	payload := mustJSON(t, map[string]any{"file_id": "file-2"})
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-2", TenantID: "school-a", EventType: "file.deleted.v1", Payload: payload, CleanupPath: "path"}}}
	if err := dispatch(context.Background(), repo, &fakeStorage{err: errors.New("storage down")}, fakePublisher{}, logger(), metrics()); err != nil {
		t.Fatal(err)
	}
	if repo.failed["event-2"] != "storage down" || len(repo.published) != 0 {
		t.Fatalf("cleanup failure=%v", repo.failed)
	}
	repo = &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-3", TenantID: "school-a", EventType: "file.updated.v1", Payload: payload}}}
	if err := dispatch(context.Background(), repo, &fakeStorage{}, fakePublisher{err: errors.New("broker down")}, logger(), metrics()); err != nil {
		t.Fatal(err)
	}
	if repo.failed["event-3"] != "broker down" {
		t.Fatalf("broker failure=%v", repo.failed)
	}
}

func TestDispatchRejectsMalformedPayload(t *testing.T) {
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-4", Payload: json.RawMessage(`{`)}}}
	if err := dispatch(context.Background(), repo, &fakeStorage{}, fakePublisher{}, logger(), metrics()); err != nil {
		t.Fatal(err)
	}
	if repo.failed["event-4"] != "invalid outbox payload" {
		t.Fatalf("failed=%v", repo.failed)
	}
}
