package workercmd

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	identityevents "github.com/auraedu/identity-service/internal/adapters/events"
	"github.com/auraedu/identity-service/internal/adapters/postgres"
	"github.com/auraedu/identity-service/internal/application"
	identitydb "github.com/auraedu/identity-service/internal/db"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

func TestLiveRoleChangeOutboxPublishesOnce(t *testing.T) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		t.Skip("NATS_URL is required for live outbox proof")
	}
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "")
	pool, err := identitydb.Open(ctx, tdb.DSN)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer pool.Close()
	if err := identitydb.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	nc, err := nats.Connect(natsURL, nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		t.Fatalf("ensure stream: %v", err)
	}
	subscription, err := js.SubscribeSync("AURA.user.role_changed.v1", nats.DeliverNew(), nats.AckExplicit())
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Unsubscribe() //nolint:errcheck // Cleanup is best effort and the primary operation determines the result.

	repo := postgres.NewRepository(pool)
	direct := identityevents.NewRecordingPublisher()
	svc := application.NewService(repo, nil, direct, []byte("live-signing-key"), time.Hour, 7*24*time.Hour)
	actor := auth.Actor{
		UserID: "admin-" + uuid.NewString(), TenantID: "upshs", Role: "school_admin",
		Permissions: []string{"users.create", "users.read", "users.update", "roles.assign", "students.read", "staff.read"},
	}
	user, err := svc.CreateUser(ctx, actor, application.CreateUserInput{
		Email: "live-" + uuid.NewString() + "@upshs.example", Name: "Private Name", Role: "teacher",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	permissions := []string{"students.read", "staff.read"}
	if _, err := svc.AssignRole(ctx, actor, user.ID, "principal", permissions); err != nil {
		t.Fatalf("assign role: %v", err)
	}
	if _, err := svc.AssignRole(ctx, actor, user.ID, "principal", permissions); err != nil {
		t.Fatalf("idempotent assign role: %v", err)
	}
	if len(direct.Events) != 0 {
		t.Fatalf("postgres path used direct publisher: %+v", direct.Events)
	}

	publisher := identityevents.NewPublisher(eventbus.NewPublisher(js))
	metrics := observ.NewWorkerMetrics("identity-worker-live-test", "outbox-batch", "outbox-publish")
	if err := dispatchOutbox(ctx, repo, publisher, metrics); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	message, err := subscription.NextMsg(5 * time.Second)
	if err != nil {
		t.Fatalf("receive role event: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(message.Data, &envelope); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if envelope["type"] != "user.role_changed.v1" || envelope["tenant_id"] != "upshs" {
		t.Fatalf("envelope=%v", envelope)
	}
	if message.Header.Get("Nats-Msg-Id") == "" || envelope["id"] != message.Header.Get("Nats-Msg-Id") {
		t.Fatalf("event id and JetStream idempotency key differ: event=%v header=%q", envelope["id"], message.Header.Get("Nats-Msg-Id"))
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("event data has unexpected type: %#v", envelope["data"])
	}
	if data["user_id"] != user.ID || data["previous_role"] != "teacher" || data["new_role"] != "principal" {
		t.Fatalf("data=%v", data)
	}
	for _, forbidden := range []string{"email", "name", "password", "token"} {
		if _, found := data[forbidden]; found {
			t.Fatalf("role event leaked %s", forbidden)
		}
	}
	if err := message.Ack(); err != nil {
		t.Fatalf("ack: %v", err)
	}

	if err := dispatchOutbox(ctx, repo, publisher, metrics); err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	pending, err := repo.ClaimPending(ctx, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("published outbox must be drained: pending=%+v err=%v", pending, err)
	}
}
