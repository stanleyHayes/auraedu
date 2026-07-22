package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/auraedu/ai-orchestrator-service/internal/adapters/postgres"
	"github.com/auraedu/ai-orchestrator-service/internal/application"
	"github.com/auraedu/ai-orchestrator-service/internal/domain"
	"github.com/auraedu/platform/auth"
	platformdb "github.com/auraedu/platform/db"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
)

type knowledgeStub struct{}
type emptyKnowledgeStub struct{}

type actionExecutorStub struct{ calls int }

func (e *actionExecutorStub) Execute(_ context.Context, action domain.ActionProposal, _ auth.Actor) (domain.ActionExecutionResult, error) {
	e.calls++
	return domain.ActionExecutionResult{StatusCode: 200, Body: json.RawMessage(`{"id":"` + action.TargetID + `","owner_user_id":"22222222-2222-4222-8222-222222222222"}`)}, nil
}

func (knowledgeStub) Search(context.Context, string, string, string, int, time.Time) ([]domain.KnowledgeResult, error) {
	return []domain.KnowledgeResult{{SourceID: "e2d2d539-91fe-48a9-85ec-2f77407211a2", Title: "Admissions Guide",
		Passage: "Applications are submitted through the official applicant portal.", Locale: "en", Version: 1, Score: 0.7}}, nil
}

func (emptyKnowledgeStub) Search(context.Context, string, string, string, int, time.Time) ([]domain.KnowledgeResult, error) {
	return nil, nil
}

func TestPostgresAssistantEscalationIsAtomicAndPrivate(t *testing.T) {
	ctx := context.Background()
	database := testkit.NewPostgres(ctx, t, "../../migrations").DB
	repo := postgres.NewRepository(database)
	svc := application.NewService(repo, emptyKnowledgeStub{})
	first, err := svc.Ask(ctx, application.AskInput{TenantID: "school-one", IdempotencyKey: "assistant-escalation-key-0001", Question: "Can somebody confirm my application?"})
	if err != nil || !first.NeedsHuman {
		t.Fatalf("create escalation: response=%+v err=%v", first, err)
	}
	events, err := repo.ClaimPending(ctx, 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("durable escalation events=%+v err=%v", events, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(events[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["message_id"] != first.MessageID || payload["session_id"] != first.SessionID {
		t.Fatalf("event identity=%+v", payload)
	}
	for _, forbidden := range []string{"question", "answer", "escalation_message"} {
		if _, exposed := payload[forbidden]; exposed {
			t.Fatalf("escalation event leaked %s: %+v", forbidden, payload)
		}
	}
	if _, err := database.Pool().Exec(ctx, `DROP TABLE assistant_outbox`); err != nil {
		t.Fatalf("remove outbox: %v", err)
	}
	key := "assistant-escalation-key-0002"
	if _, err := svc.Ask(ctx, application.AskInput{TenantID: "school-one", IdempotencyKey: key, Question: "I still need a human response"}); err == nil {
		t.Fatal("exchange must fail when its escalation cannot be committed")
	}
	sum := sha256.Sum256([]byte(key))
	if _, _, found, err := repo.FindReplay(ctx, "school-one", hex.EncodeToString(sum[:]), ""); err != nil || found {
		t.Fatalf("exchange committed without escalation: found=%v err=%v", found, err)
	}
}

func TestPostgresAssistantIdempotencyAndTenantIsolation(t *testing.T) {
	ctx := context.Background()
	var database *platformdb.DB
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		var err error
		database, err = platformdb.Open(ctx, platformdb.Config{DSN: dsn, Migrations: "../../migrations"})
		if err != nil {
			t.Fatalf("open test database: %v", err)
		}
		t.Cleanup(database.Close)
	} else {
		database = testkit.NewPostgres(ctx, t, "../../migrations").DB
	}
	svc := application.NewService(postgres.NewRepository(database), knowledgeStub{})
	first, err := svc.Ask(ctx, application.AskInput{TenantID: "school-one", IdempotencyKey: "assistant-integration-key-0001", Question: "How do I apply?"})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	replay, err := svc.Ask(ctx, application.AskInput{TenantID: "school-one", IdempotencyKey: "assistant-integration-key-0001", Question: "How do I apply?"})
	if err != nil || replay.MessageID != first.MessageID {
		t.Fatalf("replay: response=%+v err=%v", replay, err)
	}
	other, err := svc.Ask(ctx, application.AskInput{TenantID: "school-two", IdempotencyKey: "assistant-integration-key-0001", Question: "How do I apply?"})
	if err != nil || other.MessageID == first.MessageID {
		t.Fatalf("tenant-isolated idempotency: response=%+v err=%v", other, err)
	}
	maintenance := auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	if _, err := database.Exec(maintenance, `UPDATE assistant_exchanges SET expires_at=now()-interval '1 minute' WHERE message_id=$1`, first.MessageID); err != nil {
		t.Fatalf("expire exchange: %v", err)
	}
	deleted, err := postgres.NewRepository(database).PurgeExpired(ctx)
	if err != nil || deleted != 1 {
		t.Fatalf("retention cleanup: deleted=%d err=%v", deleted, err)
	}

	repo := postgres.NewRepository(database)
	gate := flags.NewStaticSnapshot()
	gate.Set("school-one", application.FeatureAutonomousActions, true)
	executor := &actionExecutorStub{}
	actionSvc := application.NewActionService(repo, executor, gate)
	proposer := auth.Actor{UserID: "ai-agent-integration", TenantID: "school-one", Role: "ai_agent", Permissions: []string{application.PermConfigureAgent}}
	reviewer := auth.Actor{UserID: "admin-integration", TenantID: "school-one", Role: "school_admin", Permissions: []string{application.PermConfigureAgent, application.PermApproveAction, application.PermAssignLead}}
	actorContext := func(actor auth.Actor, tenantID string) context.Context {
		value := tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID, ActorID: actor.UserID, ActorRole: actor.Role})
		return auth.WithActor(value, actor)
	}
	action, err := actionSvc.Propose(actorContext(proposer, "school-one"), proposer, application.ProposeActionInput{Action: domain.ActionCRMAssignLead,
		TargetID: "11111111-1111-4111-8111-111111111111", Payload: json.RawMessage(`{"owner_user_id":"22222222-2222-4222-8222-222222222222"}`),
		Reason: "Assign the qualified integration lead to the available owner.", IdempotencyKey: "action-integration-key-0001"})
	if err != nil || action.Status != domain.ActionPending {
		t.Fatalf("propose controlled action: action=%+v err=%v", action, err)
	}
	completed, err := actionSvc.Review(actorContext(reviewer, "school-one"), reviewer, action.ID, "Verified owner availability.", true)
	if err != nil || completed.Status != domain.ActionSucceeded || executor.calls != 1 {
		t.Fatalf("execute controlled action: action=%+v calls=%d err=%v", completed, executor.calls, err)
	}
	if otherTenantActions, err := repo.ListActions(actorContext(reviewer, "school-two"), "school-two", "", 20); err != nil || len(otherTenantActions) != 0 {
		t.Fatalf("cross-tenant action visibility: actions=%+v err=%v", otherTenantActions, err)
	}
	_, audit, err := actionSvc.Get(actorContext(reviewer, "school-one"), reviewer, action.ID)
	if err != nil || len(audit) != 4 {
		t.Fatalf("immutable action audit: audit=%+v err=%v", audit, err)
	}
	if _, err := database.Exec(actorContext(reviewer, "school-one"), `UPDATE ai_action_audit SET event='tampered' WHERE action_id=$1`, action.ID); err == nil {
		t.Fatal("append-only action audit accepted mutation")
	}
}
