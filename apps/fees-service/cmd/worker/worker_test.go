package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/platform/tenancy"
)

type fakeFeeOutbox struct {
	items     []ports.OutboxEvent
	published []string
	failed    map[string]string
}

func (f *fakeFeeOutbox) ClaimPendingFeeEvents(context.Context, int) ([]ports.OutboxEvent, error) {
	items := f.items
	f.items = nil
	return items, nil
}
func (f *fakeFeeOutbox) MarkFeeEventPublished(_ context.Context, id string) error {
	f.published = append(f.published, id)
	return nil
}
func (f *fakeFeeOutbox) MarkFeeEventFailed(_ context.Context, id, message string) error {
	if f.failed == nil {
		f.failed = map[string]string{}
	}
	f.failed[id] = message
	return nil
}

type fakeFeePublisher struct{ err error }

func (f fakeFeePublisher) PublishWithID(context.Context, string, string, string, map[string]any) error {
	return f.err
}

func feeWorkerMetrics() *observ.WorkerMetrics {
	return observ.NewWorkerMetrics("fees-worker-test", "outbox-batch", "outbox-publish")
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestDispatchFeeOutboxPublishesAndMarks(t *testing.T) {
	payload := mustJSON(t, map[string]any{"invoice_id": "invoice-1"})
	repo := &fakeFeeOutbox{items: []ports.OutboxEvent{{ID: "event-1", TenantID: "school-a", EventType: "invoice.updated.v1", Payload: payload}}}
	if err := dispatchFeeOutbox(context.Background(), repo, fakeFeePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)), feeWorkerMetrics()); err != nil {
		t.Fatal(err)
	}
	if len(repo.published) != 1 || repo.published[0] != "event-1" || len(repo.failed) != 0 {
		t.Fatalf("unexpected dispatch state: published=%v failed=%v", repo.published, repo.failed)
	}
}

func TestDispatchFeeOutboxDefersBrokerFailure(t *testing.T) {
	payload := mustJSON(t, map[string]any{"invoice_id": "invoice-1"})
	repo := &fakeFeeOutbox{items: []ports.OutboxEvent{{ID: "event-2", TenantID: "school-a", EventType: "invoice.paid.v1", Payload: payload}}}
	brokerErr := errors.New("broker unavailable")
	if err := dispatchFeeOutbox(context.Background(), repo, fakeFeePublisher{err: brokerErr}, slog.New(slog.NewTextHandler(io.Discard, nil)), feeWorkerMetrics()); err != nil {
		t.Fatal(err)
	}
	if len(repo.published) != 0 || repo.failed["event-2"] != brokerErr.Error() {
		t.Fatalf("broker failure was not deferred: published=%v failed=%v", repo.published, repo.failed)
	}
}

func TestPaymentInputUsesCanonicalAmountAndTimestamp(t *testing.T) {
	completed := "2026-07-19T12:00:00Z"
	data := mustJSON(t, map[string]any{
		"payment_id":   "11111111-1111-4111-8111-111111111111",
		"invoice_id":   "22222222-2222-4222-8222-222222222222",
		"amount_cents": 25000,
		"amount":       1,
		"completed_at": completed,
	})
	event := tenancy.CloudEvent{SpecVersion: "1.0", ID: "event-1", Type: "payment.received.v1", Source: "payment-service", TenantID: "tenant-1", Time: time.Now().Format(time.RFC3339), Data: data}
	input, err := paymentInput(event)
	if err != nil {
		t.Fatal(err)
	}
	if input.AmountCents != 25000 || input.ReceivedAt.Format(time.RFC3339) != completed {
		t.Fatalf("input=%+v", input)
	}
}

func TestPaymentInputRejectsMalformedEvents(t *testing.T) {
	data := mustJSON(t, map[string]any{"payment_id": "p", "invoice_id": "i", "amount_cents": 0})
	event := tenancy.CloudEvent{Type: "payment.received.v1", TenantID: "tenant-1", Data: data}
	if _, err := paymentInput(event); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err=%v", err)
	}
}
