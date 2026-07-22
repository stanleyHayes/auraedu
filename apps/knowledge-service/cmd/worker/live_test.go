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

	knowledgeevents "github.com/auraedu/knowledge-service/internal/adapters/events"
	"github.com/auraedu/knowledge-service/internal/adapters/postgres"
	"github.com/auraedu/knowledge-service/internal/application"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type rejectingDirectPublisher struct{ calls int }

func (p *rejectingDirectPublisher) Publish(context.Context, string, string, map[string]any) error {
	p.calls++
	return errors.New("direct publication is forbidden for PostgreSQL approvals")
}

func TestLiveApprovalOutboxPublishesOnce(t *testing.T) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		t.Skip("NATS_URL is required for live outbox proof")
	}
	ctx := context.Background()
	database := testkit.NewPostgres(ctx, t, "../../migrations").DB
	nc, err := nats.Connect(normalizeNATSURL(natsURL), nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatalf("connect NATS: %v", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("JetStream: %v", err)
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		t.Fatalf("ensure stream: %v", err)
	}
	subscription, err := js.SubscribeSync("AURA.knowledge.source_approved.v1", nats.DeliverNew(), nats.AckExplicit())
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer func() {
		if unsubscribeErr := subscription.Unsubscribe(); unsubscribeErr != nil {
			t.Errorf("unsubscribe knowledge event test: %v", unsubscribeErr)
		}
	}()

	repo := postgres.NewRepository(database)
	direct := &rejectingDirectPublisher{}
	now := time.Now().UTC().Truncate(time.Second)
	svc := application.NewService(repo, application.WithPublisher(direct), application.WithClock(func() time.Time { return now }))
	tenantID := "school-" + uuid.NewString()
	manager := auth.Actor{UserID: "manager", TenantID: tenantID, Permissions: []string{application.PermManage}}
	reviewer := auth.Actor{UserID: "reviewer", TenantID: tenantID, Permissions: []string{application.PermApprove}}
	source, err := svc.Create(ctx, manager, application.CreateInput{
		SourceType: "fees", Title: "Verified fee schedule", Owner: "Admissions",
		Content:         "The verified application fee must be paid through the official applicant portal.",
		Confidentiality: "public", Locale: "en-GH", EffectiveAt: now,
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	if _, err := svc.Approve(ctx, reviewer, source.ID, "Checked against the signed fee schedule"); err != nil {
		t.Fatalf("approve source: %v", err)
	}
	if direct.calls != 0 {
		t.Fatalf("PostgreSQL approval used direct publisher %d times", direct.calls)
	}

	publisher := knowledgeevents.NewPublisher(eventbus.NewPublisher(js))
	metrics := observ.NewWorkerMetrics("knowledge-worker-live-test", "outbox-publish")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := dispatch(ctx, repo, publisher, log, metrics); err != nil {
		t.Fatalf("dispatch outbox: %v", err)
	}
	message, err := subscription.NextMsg(5 * time.Second)
	if err != nil {
		t.Fatalf("receive approval event: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(message.Data, &envelope); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if envelope["type"] != "knowledge.source_approved.v1" || envelope["tenant_id"] != tenantID || envelope["subject"] != source.ID {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if message.Header.Get("Nats-Msg-Id") == "" || envelope["id"] != message.Header.Get("Nats-Msg-Id") {
		t.Fatalf("event and broker identities differ: event=%v header=%q", envelope["id"], message.Header.Get("Nats-Msg-Id"))
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("event data has unexpected type: %#v", envelope["data"])
	}
	if data["source_id"] != source.ID || data["locale"] != "en-GH" {
		t.Fatalf("unexpected event data: %+v", data)
	}
	for _, forbidden := range []string{"content", "owner", "approved_by", "review_note"} {
		if _, exposed := data[forbidden]; exposed {
			t.Fatalf("approval event leaked %s", forbidden)
		}
	}
	if err := message.Ack(); err != nil {
		t.Fatalf("ack: %v", err)
	}
	if err := dispatch(ctx, repo, publisher, log, metrics); err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	pending, err := repo.ClaimPending(ctx, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("published outbox must be drained: pending=%+v err=%v", pending, err)
	}
}
