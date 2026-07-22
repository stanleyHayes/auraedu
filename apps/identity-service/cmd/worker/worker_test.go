package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/auraedu/identity-service/internal/ports"
	"github.com/auraedu/platform/observ"
)

type fakeOutbox struct {
	items     []ports.OutboxEvent
	published []string
	failed    map[string]string
}

func (f *fakeOutbox) ClaimPending(context.Context, int) ([]ports.OutboxEvent, error) {
	items := f.items
	f.items = nil
	return items, nil
}
func (f *fakeOutbox) MarkPublished(_ context.Context, id string) error {
	f.published = append(f.published, id)
	return nil
}
func (f *fakeOutbox) MarkFailed(_ context.Context, id, message string) error {
	if f.failed == nil {
		f.failed = map[string]string{}
	}
	f.failed[id] = message
	return nil
}

type fakePublisher struct {
	events []ports.Event
	err    error
}

type fakeCleanupRepository struct {
	cutoffs ports.AuthRetentionCutoffs
	result  ports.AuthCleanupResult
	err     error
}

func (f *fakeCleanupRepository) CleanupAuthArtifacts(_ context.Context, cutoffs ports.AuthRetentionCutoffs) (ports.AuthCleanupResult, error) {
	f.cutoffs = cutoffs
	return f.result, f.err
}

func (p *fakePublisher) Publish(_ context.Context, event ports.Event) error {
	if p.err != nil {
		return p.err
	}
	p.events = append(p.events, event)
	return nil
}

func workerMetrics() *observ.WorkerMetrics {
	return observ.NewWorkerMetrics("identity-worker-test", "outbox-batch", "outbox-publish", "auth-cleanup")
}

func TestLoadAuthRetentionPolicyDefaults(t *testing.T) {
	for _, name := range []string{
		"AUTH_CLEANUP_INTERVAL",
		"AUTH_REFRESH_RETENTION_AFTER_EXPIRY",
		"AUTH_PASSWORD_RESET_RETENTION",
		"AUTH_INVITE_RETENTION",
		"AUTH_PUBLISHED_OUTBOX_RETENTION",
		"AUTH_CLEANUP_BATCH_SIZE",
	} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
	}
	policy, err := loadAuthRetentionPolicy()
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	if policy.CleanupInterval != time.Hour || policy.RefreshAfterExpiry != 24*time.Hour ||
		policy.PasswordResetRetention != 30*24*time.Hour || policy.InviteRetention != 90*24*time.Hour ||
		policy.PublishedOutboxRetention != 30*24*time.Hour {
		t.Fatalf("unexpected defaults: %+v", policy)
	}
	if policy.BatchSize != 1000 {
		t.Fatalf("batch size=%d", policy.BatchSize)
	}
}

func TestLoadAuthRetentionPolicyRejectsInvalidBatchSize(t *testing.T) {
	for _, value := range []string{"not-a-number", "0", "10001"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("AUTH_CLEANUP_BATCH_SIZE", value)
			if _, err := loadAuthRetentionPolicy(); err == nil {
				t.Fatal("expected invalid cleanup batch size to fail")
			}
		})
	}
}

func TestLoadAuthRetentionPolicyRejectsInvalidOrDisabledCleanup(t *testing.T) {
	for _, value := range []string{"not-a-duration", "0s", "-1h"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("AUTH_CLEANUP_INTERVAL", value)
			if _, err := loadAuthRetentionPolicy(); err == nil {
				t.Fatal("expected invalid cleanup interval to fail")
			}
		})
	}
}

func TestRunAuthCleanupUsesDeterministicCutoffs(t *testing.T) {
	now := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	policy := authRetentionPolicy{
		CleanupInterval:          time.Hour,
		RefreshAfterExpiry:       24 * time.Hour,
		PasswordResetRetention:   30 * 24 * time.Hour,
		InviteRetention:          90 * 24 * time.Hour,
		PublishedOutboxRetention: 30 * 24 * time.Hour,
		BatchSize:                73,
	}
	want := ports.AuthCleanupResult{RefreshTokens: 2, PasswordResets: 1, Invites: 3, OutboxEvents: 4}
	repo := &fakeCleanupRepository{result: want}
	got, err := runAuthCleanup(context.Background(), repo, policy, now)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if got != want {
		t.Fatalf("result=%+v want=%+v", got, want)
	}
	if !repo.cutoffs.RefreshFamiliesBefore.Equal(now.Add(-24*time.Hour)) ||
		!repo.cutoffs.PasswordResetsBefore.Equal(now.Add(-30*24*time.Hour)) ||
		!repo.cutoffs.InvitesBefore.Equal(now.Add(-90*24*time.Hour)) ||
		!repo.cutoffs.PublishedOutboxBefore.Equal(now.Add(-30*24*time.Hour)) {
		t.Fatalf("unexpected cutoffs: %+v", repo.cutoffs)
	}
	if repo.cutoffs.BatchSize != 73 {
		t.Fatalf("batch size=%d", repo.cutoffs.BatchSize)
	}
}

func TestDispatchOutboxPublishesStableEventAndMarksIt(t *testing.T) {
	payload, err := json.Marshal(map[string]any{"user_id": "user-1", "new_role": "principal"})
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeOutbox{items: []ports.OutboxEvent{{
		ID: "11111111-1111-4111-8111-111111111111", TenantID: "upshs",
		EventType: "user.role_changed.v1", Payload: payload, CreatedAt: time.Now(),
	}}}
	publisher := &fakePublisher{}
	if err := dispatchOutbox(context.Background(), repo, publisher, workerMetrics()); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(publisher.events) != 1 || len(repo.published) != 1 || publisher.events[0].ID != repo.published[0] || publisher.events[0].TenantID != "upshs" {
		t.Fatalf("published=%+v marked=%v", publisher.events, repo.published)
	}
}

func TestDispatchOutboxDefersBrokerFailure(t *testing.T) {
	repo := &fakeOutbox{items: []ports.OutboxEvent{{
		ID: "22222222-2222-4222-8222-222222222222", TenantID: "upshs",
		EventType: "user.role_changed.v1", Payload: json.RawMessage(`{"user_id":"user-1"}`),
	}}}
	if err := dispatchOutbox(context.Background(), repo, &fakePublisher{err: errors.New("broker unavailable")}, workerMetrics()); err != nil {
		t.Fatalf("dispatch should defer item failure: %v", err)
	}
	if len(repo.published) != 0 || repo.failed["22222222-2222-4222-8222-222222222222"] == "" {
		t.Fatalf("published=%v failed=%v", repo.published, repo.failed)
	}
}

func TestDispatchOutboxRecordsInvalidPayload(t *testing.T) {
	repo := &fakeOutbox{items: []ports.OutboxEvent{{
		ID: "33333333-3333-4333-8333-333333333333", TenantID: "upshs",
		EventType: "user.role_changed.v1", Payload: json.RawMessage(`{"broken"`),
	}}}
	if err := dispatchOutbox(context.Background(), repo, &fakePublisher{}, workerMetrics()); err != nil {
		t.Fatalf("dispatch invalid payload: %v", err)
	}
	if repo.failed["33333333-3333-4333-8333-333333333333"] != "invalid outbox payload" {
		t.Fatalf("failed=%v", repo.failed)
	}
}
