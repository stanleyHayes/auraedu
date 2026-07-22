package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/auraedu/payment-service/internal/ports"
	"github.com/auraedu/platform/observ"
)

type fakeOutbox struct {
	items     []ports.OutboxEvent
	published []string
	failed    map[string]string
}

func (f *fakeOutbox) ClaimPendingPaymentEvents(context.Context, int) ([]ports.OutboxEvent, error) {
	items := f.items
	f.items = nil
	return items, nil
}
func (f *fakeOutbox) MarkPaymentEventPublished(_ context.Context, id string) error {
	f.published = append(f.published, id)
	return nil
}
func (f *fakeOutbox) MarkPaymentEventFailed(_ context.Context, id, message string) error {
	if f.failed == nil {
		f.failed = map[string]string{}
	}
	f.failed[id] = message
	return nil
}

type fakeOutboxPublisher struct{ err error }

func (f fakeOutboxPublisher) PublishWithID(context.Context, string, string, string, map[string]any) error {
	return f.err
}

func testMetrics() *observ.WorkerMetrics {
	return observ.NewWorkerMetrics("payment-worker-test", "outbox-batch", "outbox-publish")
}

func mustMarshalPayload(t *testing.T, value map[string]any) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal outbox payload: %v", err)
	}
	return payload
}

func TestDispatchMarksPublishedAfterSuccessfulSend(t *testing.T) {
	payload := mustMarshalPayload(t, map[string]any{"payment_id": "pay-1"})
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-1", TenantID: "school-a", EventType: "payment.received.v1", Payload: payload, CreatedAt: time.Now()}}}
	if err := dispatch(context.Background(), repo, fakeOutboxPublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)), testMetrics()); err != nil {
		t.Fatal(err)
	}
	if len(repo.published) != 1 || repo.published[0] != "event-1" || len(repo.failed) != 0 {
		t.Fatalf("unexpected dispatch state: published=%v failed=%v", repo.published, repo.failed)
	}
}

func TestDispatchDefersBrokerFailure(t *testing.T) {
	payload := mustMarshalPayload(t, map[string]any{"payment_id": "pay-1"})
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-2", TenantID: "school-a", EventType: "payment.received.v1", Payload: payload}}}
	brokerErr := errors.New("broker unavailable")
	if err := dispatch(context.Background(), repo, fakeOutboxPublisher{err: brokerErr}, slog.New(slog.NewTextHandler(io.Discard, nil)), testMetrics()); err != nil {
		t.Fatal(err)
	}
	if len(repo.published) != 0 || repo.failed["event-2"] != brokerErr.Error() {
		t.Fatalf("broker failure was not deferred: published=%v failed=%v", repo.published, repo.failed)
	}
}

func TestDispatchRejectsInvalidPayloadWithoutPublishing(t *testing.T) {
	repo := &fakeOutbox{items: []ports.OutboxEvent{{ID: "event-3", Payload: json.RawMessage(`{`)}}}
	if err := dispatch(context.Background(), repo, fakeOutboxPublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)), testMetrics()); err != nil {
		t.Fatal(err)
	}
	if len(repo.published) != 0 || repo.failed["event-3"] != "invalid outbox payload" {
		t.Fatalf("invalid payload handling: published=%v failed=%v", repo.published, repo.failed)
	}
}
