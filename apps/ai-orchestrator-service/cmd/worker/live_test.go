package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	assistantevents "github.com/auraedu/ai-orchestrator-service/internal/adapters/events"
	"github.com/auraedu/ai-orchestrator-service/internal/adapters/postgres"
	"github.com/auraedu/ai-orchestrator-service/internal/application"
	"github.com/auraedu/ai-orchestrator-service/internal/domain"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type emptyRetriever struct{}

func (emptyRetriever) Search(context.Context, string, string, string, int, time.Time) ([]domain.KnowledgeResult, error) {
	return nil, nil
}

type rejectingPublisher struct{ calls int }

func (p *rejectingPublisher) Publish(context.Context, string, string, map[string]any) error {
	p.calls++
	return errors.New("direct publication forbidden")
}

func TestLiveEscalationOutboxPublishesOnce(t *testing.T) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		t.Skip("NATS_URL is required for live outbox proof")
	}
	ctx := context.Background()
	database := testkit.NewPostgres(ctx, t, "../../migrations").DB
	nc, err := nats.Connect(normalizeNATSURL(natsURL), nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		t.Fatal(err)
	}
	sub, err := js.SubscribeSync("AURA.assistant.question_unanswered.v1", nats.DeliverNew(), nats.AckExplicit())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if unsubscribeErr := sub.Unsubscribe(); unsubscribeErr != nil {
			t.Errorf("unsubscribe: %v", unsubscribeErr)
		}
	}()
	repo := postgres.NewRepository(database)
	direct := &rejectingPublisher{}
	svc := application.NewService(repo, emptyRetriever{}, application.WithPublisher(direct))
	tenantID := "school-" + uuid.NewString()
	response, err := svc.Ask(ctx, application.AskInput{TenantID: tenantID, IdempotencyKey: "live-assistant-escalation-key", Question: "Please ask admissions to contact me"})
	if err != nil || !response.NeedsHuman || direct.calls != 0 {
		t.Fatalf("response=%+v direct_calls=%d err=%v", response, direct.calls, err)
	}
	metrics := observ.NewWorkerMetrics("assistant-worker-live-test", "outbox-publish")
	if err := dispatch(ctx, repo, assistantevents.NewPublisher(eventbus.NewPublisher(js)), slog.New(slog.NewTextHandler(io.Discard, nil)), metrics); err != nil {
		t.Fatal(err)
	}
	message, err := sub.NextMsg(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(message.Data, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["tenant_id"] != tenantID || envelope["subject"] != response.MessageID || envelope["id"] != message.Header.Get("Nats-Msg-Id") {
		t.Fatalf("identity mismatch: envelope=%+v header=%q", envelope, message.Header.Get("Nats-Msg-Id"))
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("event data is not an object: %+v", envelope["data"])
	}
	for _, forbidden := range []string{"question", "answer", "escalation_message"} {
		if _, exposed := data[forbidden]; exposed {
			t.Fatalf("event leaked %s: %+v", forbidden, data)
		}
	}
	if err := message.Ack(); err != nil {
		t.Fatal(err)
	}
	if err := dispatch(ctx, repo, assistantevents.NewPublisher(eventbus.NewPublisher(js)), slog.New(slog.NewTextHandler(io.Discard, nil)), metrics); err != nil {
		t.Fatal(err)
	}
	if pending, err := repo.ClaimPending(ctx, 10); err != nil || len(pending) != 0 {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
}
