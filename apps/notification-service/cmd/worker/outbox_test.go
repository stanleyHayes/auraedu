package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/observ"
)

type fakeNotificationOutbox struct {
	items     []ports.OutboxEvent
	published []string
	failed    map[string]string
}

func (f *fakeNotificationOutbox) ClaimPendingNotificationEvents(context.Context, int) ([]ports.OutboxEvent, error) {
	items := f.items
	f.items = nil
	return items, nil
}
func (f *fakeNotificationOutbox) MarkNotificationEventPublished(_ context.Context, id string) error {
	f.published = append(f.published, id)
	return nil
}
func (f *fakeNotificationOutbox) MarkNotificationEventFailed(_ context.Context, id, message string) error {
	if f.failed == nil {
		f.failed = map[string]string{}
	}
	f.failed[id] = message
	return nil
}

type fakeNotificationPublisher struct{ err error }

func (f fakeNotificationPublisher) PublishWithID(context.Context, string, string, string, map[string]any) error {
	return f.err
}

func notificationOutboxMetrics() *observ.WorkerMetrics {
	return observ.NewWorkerMetrics("notification-worker-test", "outbox-batch", "outbox-publish")
}

func mustMarshalOutboxPayload(t *testing.T, value map[string]any) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal outbox payload: %v", err)
	}
	return payload
}

func TestDispatchNotificationOutboxPublishesAndMarks(t *testing.T) {
	payload := mustMarshalOutboxPayload(t, map[string]any{"message_id": "message-1"})
	repo := &fakeNotificationOutbox{items: []ports.OutboxEvent{{ID: "event-1", TenantID: "school-a", EventType: "notification.sent.v1", Payload: payload}}}
	if err := dispatchNotificationOutbox(context.Background(), repo, fakeNotificationPublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)), notificationOutboxMetrics()); err != nil {
		t.Fatal(err)
	}
	if len(repo.published) != 1 || repo.published[0] != "event-1" || len(repo.failed) != 0 {
		t.Fatalf("unexpected dispatch state: published=%v failed=%v", repo.published, repo.failed)
	}
}

func TestDispatchNotificationOutboxDefersBrokerFailure(t *testing.T) {
	payload := mustMarshalOutboxPayload(t, map[string]any{"message_id": "message-2"})
	repo := &fakeNotificationOutbox{items: []ports.OutboxEvent{{ID: "event-2", TenantID: "school-a", EventType: "notification.failed.v1", Payload: payload}}}
	brokerErr := errors.New("broker unavailable")
	if err := dispatchNotificationOutbox(context.Background(), repo, fakeNotificationPublisher{err: brokerErr}, slog.New(slog.NewTextHandler(io.Discard, nil)), notificationOutboxMetrics()); err != nil {
		t.Fatal(err)
	}
	if len(repo.published) != 0 || repo.failed["event-2"] != brokerErr.Error() {
		t.Fatalf("broker failure was not deferred: published=%v failed=%v", repo.published, repo.failed)
	}
}

func TestDispatchNotificationOutboxRejectsMalformedPayload(t *testing.T) {
	repo := &fakeNotificationOutbox{items: []ports.OutboxEvent{{ID: "event-3", Payload: json.RawMessage(`{`)}}}
	if err := dispatchNotificationOutbox(context.Background(), repo, fakeNotificationPublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)), notificationOutboxMetrics()); err != nil {
		t.Fatal(err)
	}
	if len(repo.published) != 0 || repo.failed["event-3"] != "invalid outbox payload" {
		t.Fatalf("malformed payload handling: published=%v failed=%v", repo.published, repo.failed)
	}
}
