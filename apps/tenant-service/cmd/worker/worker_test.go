package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/auraedu/platform/observ"
	"github.com/auraedu/tenant-service/internal/ports"
)

type fakeOutboxRepository struct {
	items     []ports.OutboxEvent
	published []string
	failed    map[string]string
}

func (r *fakeOutboxRepository) ClaimPending(context.Context, int) ([]ports.OutboxEvent, error) {
	return r.items, nil
}
func (r *fakeOutboxRepository) MarkPublished(_ context.Context, id string) error {
	r.published = append(r.published, id)
	return nil
}
func (r *fakeOutboxRepository) MarkFailed(_ context.Context, id, message string) error {
	if r.failed == nil {
		r.failed = map[string]string{}
	}
	r.failed[id] = message
	return nil
}

type publishedOutboxEvent struct {
	ID, EventType, TenantID string
	Payload                 map[string]any
}

type fakeOutboxPublisher struct {
	err    error
	events []publishedOutboxEvent
}

func (p *fakeOutboxPublisher) PublishWithID(_ context.Context, id, eventType, tenantID string, payload map[string]any) error {
	if p.err != nil {
		return p.err
	}
	p.events = append(p.events, publishedOutboxEvent{ID: id, EventType: eventType, TenantID: tenantID, Payload: payload})
	return nil
}

func workerTestMetrics() *observ.WorkerMetrics {
	return observ.NewWorkerMetrics("tenant-service-worker-test", "outbox-batch", "outbox-publish")
}

func TestDispatchPublishesStableEventAndMarksComplete(t *testing.T) {
	payload, err := json.Marshal(map[string]any{"tenant_code": "readiness-academy", "plan": "growth"})
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeOutboxRepository{items: []ports.OutboxEvent{{
		ID: "11111111-1111-4111-8111-111111111111", TenantID: "readiness-academy",
		EventType: "tenant.created.v1", Payload: payload,
	}}}
	publisher := &fakeOutboxPublisher{}
	if err := dispatch(context.Background(), repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), workerTestMetrics()); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(publisher.events) != 1 || publisher.events[0].ID != repo.items[0].ID || publisher.events[0].TenantID != "readiness-academy" {
		t.Fatalf("published events=%+v", publisher.events)
	}
	if len(repo.published) != 1 || repo.published[0] != repo.items[0].ID || len(repo.failed) != 0 {
		t.Fatalf("published=%v failed=%v", repo.published, repo.failed)
	}
}

func TestDispatchDefersBrokerFailureWithoutMarkingPublished(t *testing.T) {
	repo := &fakeOutboxRepository{items: []ports.OutboxEvent{{
		ID: "22222222-2222-4222-8222-222222222222", TenantID: "readiness-academy",
		EventType: "tenant.created.v1", Payload: json.RawMessage(`{"tenant_code":"readiness-academy"}`),
	}}}
	publisher := &fakeOutboxPublisher{err: errors.New("broker unavailable")}
	if err := dispatch(context.Background(), repo, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), workerTestMetrics()); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(repo.published) != 0 || repo.failed[repo.items[0].ID] != "broker unavailable" {
		t.Fatalf("published=%v failed=%v", repo.published, repo.failed)
	}
}

func TestDispatchQuarantinesInvalidPayload(t *testing.T) {
	repo := &fakeOutboxRepository{items: []ports.OutboxEvent{{
		ID: "33333333-3333-4333-8333-333333333333", TenantID: "readiness-academy",
		EventType: "tenant.created.v1", Payload: json.RawMessage(`{"broken"`),
	}}}
	if err := dispatch(context.Background(), repo, &fakeOutboxPublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)), workerTestMetrics()); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(repo.published) != 0 || repo.failed[repo.items[0].ID] != "invalid outbox payload" {
		t.Fatalf("published=%v failed=%v", repo.published, repo.failed)
	}
}
