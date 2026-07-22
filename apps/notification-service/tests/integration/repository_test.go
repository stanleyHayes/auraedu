package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/auraedu/notification-service/internal/adapters/notifier"
	"github.com/auraedu/notification-service/internal/adapters/postgres"
	"github.com/auraedu/notification-service/internal/application"
	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestDeliveryOutcomeAtomicallyEnqueuesTenantEvent(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewMessageRepository(tdb.DB)
	message := mustCreateMessage(ctx, t, repo, recipientA, "in_app", "Delivery proof")
	svc := application.NewService(repo, nil, nil, application.WithNotifiers(map[string]ports.Notifier{"in_app": notifier.InboxNotifier{}}))
	if err := svc.DeliverScheduled(ctx, message); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.GetByID(ctx, tenantA, message.ID)
	if err != nil || stored.Status != string(domain.MessageStatusSent) {
		t.Fatalf("stored outcome=%+v err=%v", stored, err)
	}
	outbox, err := repo.ClaimPendingNotificationEvents(context.Background(), 10)
	if err != nil || len(outbox) != 1 {
		t.Fatalf("delivery outbox=%+v err=%v", outbox, err)
	}
	if outbox[0].TenantID != tenantA || outbox[0].EventType != "notification.sent.v1" {
		t.Fatalf("unexpected delivery event: %+v", outbox[0])
	}
	var got map[string]any
	if err := json.Unmarshal(outbox[0].Payload, &got); err != nil || got["message_id"] != message.ID {
		t.Fatalf("delivery payload=%+v err=%v", got, err)
	}
	if err := repo.MarkNotificationEventPublished(context.Background(), outbox[0].ID); err != nil {
		t.Fatal(err)
	}
	if pending, err := repo.ClaimPendingNotificationEvents(context.Background(), 10); err != nil || len(pending) != 0 {
		t.Fatalf("published outbox must drain: pending=%+v err=%v", pending, err)
	}
}

func TestDeliveryOutcomeRollsBackWithoutOutbox(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewMessageRepository(tdb.DB)
	message := mustCreateMessage(ctx, t, repo, recipientA, "in_app", "Rollback proof")
	message.MarkSent()
	if _, err := tdb.DB.Pool().Exec(context.Background(), `DROP TABLE notification_outbox`); err != nil {
		t.Fatal(err)
	}
	if err := repo.CommitDeliveryOutcome(ctx, tenantA, message, "", false, "notification.sent.v1", map[string]any{"message_id": message.ID}); err == nil {
		t.Fatal("delivery state must fail when its durable event cannot be written")
	}
	stored, err := repo.GetByID(ctx, tenantA, message.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != string(domain.MessageStatusPending) || stored.SentAt != nil {
		t.Fatalf("delivery mutation escaped rollback: %+v", stored)
	}
}

func TestScheduledMessageClaimAndCancellation(t *testing.T) {
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewMessageRepository(tdb.DB)
	past := time.Now().UTC().Add(-time.Minute)
	message, err := domain.NewMessage(tenantA, recipientA, "in_app", "Offer reminder", "Review your offer", nil, map[string]any{"application_id": "app-1"}, &past)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.Create(withTenant(context.Background(), tenantA), tenantA, message); err != nil {
		t.Fatal(err)
	}
	due, err := repo.ClaimDue(context.Background(), 10, 5*time.Minute)
	if err != nil || len(due) != 1 || due[0].TenantID != tenantA {
		t.Fatalf("claim due=%+v err=%v", due, err)
	}
	if err = repo.CancelByApplication(withTenant(context.Background(), tenantA), tenantA, "app-1"); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(withTenant(context.Background(), tenantA), tenantA, message.ID)
	if err != nil || got.Status != string(domain.MessageStatusCancelled) {
		t.Fatalf("cancelled message=%+v err=%v", got, err)
	}
}

func TestDeviceTokenUpsertTransferAndTenantIsolation(t *testing.T) {
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewDeviceTokenRepository(tdb.DB)
	device, err := domain.NewDeviceToken(tenantA, recipientA, "phone-1", "android", "ExponentPushToken[test-device-token]")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := repo.Upsert(withTenant(context.Background(), tenantA), tenantA, device)
	if err != nil || stored.Status != "active" {
		t.Fatalf("upsert=%+v err=%v", stored, err)
	}
	list, err := repo.ListActive(withTenant(context.Background(), tenantA), tenantA, recipientA)
	if err != nil || len(list) != 1 {
		t.Fatalf("active list=%+v err=%v", list, err)
	}
	otherTenant, err := repo.ListActive(withTenant(context.Background(), tenantB), tenantB, recipientA)
	if err != nil || len(otherTenant) != 0 {
		t.Fatalf("tenant isolation list=%+v err=%v", otherTenant, err)
	}
	if err := repo.MarkInvalid(withTenant(context.Background(), tenantA), tenantA, device.Token); err != nil {
		t.Fatal(err)
	}
	list, err = repo.ListActive(withTenant(context.Background(), tenantA), tenantA, recipientA)
	if err != nil || len(list) != 0 {
		t.Fatalf("invalid token remained active: %+v err=%v", list, err)
	}
}

const (
	tenantA = "11111111-1111-1111-1111-111111111111"
	tenantB = "22222222-2222-2222-2222-222222222222"

	recipientA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	recipientB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

func newMessageRepo(t *testing.T) ports.MessageRepository {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewMessageRepository(tdb.DB)
}

func newTemplateRepo(t *testing.T) ports.TemplateRepository {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewTemplateRepository(tdb.DB)
}

func newSubscriptionRepo(t *testing.T) ports.SubscriptionRepository {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewSubscriptionRepository(tdb.DB)
}

func newAllRepos(t *testing.T) (ports.MessageRepository, ports.TemplateRepository, ports.SubscriptionRepository) {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewMessageRepository(tdb.DB), postgres.NewTemplateRepository(tdb.DB), postgres.NewSubscriptionRepository(tdb.DB)
}

func withTenant(ctx context.Context, tenantA string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantA})
}

func actorWithPerms(tenantA string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantA, Permissions: perms}
}

func mustCreateMessage(ctx context.Context, t *testing.T, repo ports.MessageRepository, recipientID, channel, subject string) *domain.Message {
	t.Helper()
	m, err := domain.NewMessage(tenantA, recipientID, channel, subject, "Body", nil, nil, nil)
	if err != nil {
		t.Fatalf("new message: %v", err)
	}
	if err := repo.Create(ctx, tenantA, m); err != nil {
		t.Fatalf("create message: %v", err)
	}
	return m
}

func mustCreateTemplate(ctx context.Context, t *testing.T, repo ports.TemplateRepository, name, channel string) *domain.Template {
	t.Helper()
	tmpl, err := domain.NewTemplate(tenantA, name, channel, "Subject", "Body")
	if err != nil {
		t.Fatalf("new template: %v", err)
	}
	if err := repo.Create(ctx, tenantA, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	return tmpl
}

func mustCreateSubscription(ctx context.Context, t *testing.T, repo ports.SubscriptionRepository, userID, channel string) *domain.Subscription {
	t.Helper()
	sub, err := domain.NewSubscription(tenantA, userID, channel, true)
	if err != nil {
		t.Fatalf("new subscription: %v", err)
	}
	if err := repo.Create(ctx, tenantA, sub); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	return sub
}

func TestMessageRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newMessageRepo(t)

	m := mustCreateMessage(ctx, t, repo, recipientA, "email", "Hello")

	got, err := repo.GetByID(ctx, tenantA, m.ID)
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if got.ID != m.ID || got.RecipientID != recipientA {
		t.Fatalf("message mismatch: %+v", got)
	}
}

func TestMessageRepository_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newMessageRepo(t)

	mustCreateMessage(ctx, t, repo, recipientA, "email", "Hello")
	m2 := mustCreateMessage(ctx, t, repo, recipientA, "email", "Hello 2")

	page, next, err := repo.List(ctx, tenantA, ports.MessageFilter{Limit: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	page2, _, err := repo.List(ctx, tenantA, ports.MessageFilter{Limit: 1, Cursor: next})
	if err != nil {
		t.Fatalf("list cursor: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != m2.ID {
		t.Fatalf("expected second message, got %+v", page2)
	}
}

func TestMessageRepository_ListFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newMessageRepo(t)

	mustCreateMessage(ctx, t, repo, recipientA, "email", "Hello")
	mustCreateMessage(ctx, t, repo, recipientB, "sms", "SMS")

	cases := []struct {
		name   string
		filter ports.MessageFilter
		want   int
	}{
		{"by recipient_id", ports.MessageFilter{Limit: 10, RecipientID: recipientA}, 1},
		{"by channel", ports.MessageFilter{Limit: 10, Channel: "sms"}, 1},
		{"by status", ports.MessageFilter{Limit: 10, Status: string(domain.MessageStatusPending)}, 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, _, err := repo.List(ctx, tenantA, tc.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(page) != tc.want {
				t.Fatalf("expected %d records, got %d", tc.want, len(page))
			}
		})
	}
}

func TestMessageRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newMessageRepo(t)

	m := mustCreateMessage(ctx, t, repo, recipientA, "email", "Hello")
	subject := "Updated"
	if _, err := m.ApplyUpdate(domain.MessagePatch{Subject: &subject}); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.Update(ctx, tenantA, m); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(ctx, tenantA, m.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Subject != subject {
		t.Fatalf("message not updated: %+v", got)
	}
}

func TestMessageRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newMessageRepo(t)

	m := mustCreateMessage(ctx, t, repo, recipientA, "email", "Hello")
	if err := repo.Delete(ctx, tenantA, m.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, tenantA, m.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestTemplateRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newTemplateRepo(t)

	tmpl := mustCreateTemplate(ctx, t, repo, "welcome", "email")

	got, err := repo.GetByID(ctx, tenantA, tmpl.ID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if got.ID != tmpl.ID {
		t.Fatalf("template mismatch: %+v", got)
	}
}

func TestTemplateRepository_ListFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newTemplateRepo(t)

	mustCreateTemplate(ctx, t, repo, "welcome", "email")
	mustCreateTemplate(ctx, t, repo, "sms_alert", "sms")

	cases := []struct {
		name   string
		filter ports.TemplateFilter
		want   int
	}{
		{"by channel", ports.TemplateFilter{Limit: 10, Channel: "email"}, 1},
		{"by status", ports.TemplateFilter{Limit: 10, Status: string(domain.TemplateStatusActive)}, 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, _, err := repo.List(ctx, tenantA, tc.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(page) != tc.want {
				t.Fatalf("expected %d records, got %d", tc.want, len(page))
			}
		})
	}
}

func TestSubscriptionRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newSubscriptionRepo(t)

	sub := mustCreateSubscription(ctx, t, repo, recipientA, "email")

	got, err := repo.GetByID(ctx, tenantA, sub.ID)
	if err != nil {
		t.Fatalf("get subscription: %v", err)
	}
	if got.ID != sub.ID {
		t.Fatalf("subscription mismatch: %+v", got)
	}
}

func TestSubscriptionRepository_ListFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newSubscriptionRepo(t)

	mustCreateSubscription(ctx, t, repo, recipientA, "email")
	mustCreateSubscription(ctx, t, repo, recipientB, "sms")

	cases := []struct {
		name   string
		filter ports.SubscriptionFilter
		want   int
	}{
		{"by user_id", ports.SubscriptionFilter{Limit: 10, UserID: recipientA}, 1},
		{"by channel", ports.SubscriptionFilter{Limit: 10, Channel: "sms"}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, _, err := repo.List(ctx, tenantA, tc.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(page) != tc.want {
				t.Fatalf("expected %d records, got %d", tc.want, len(page))
			}
		})
	}
}

func TestRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	mRepo, tRepo, sRepo := newAllRepos(t)

	aCtx := withTenant(ctx, tenantA)
	m := mustCreateMessage(aCtx, t, mRepo, recipientA, "email", "Hello")
	tmpl := mustCreateTemplate(aCtx, t, tRepo, "welcome", "email")
	sub := mustCreateSubscription(aCtx, t, sRepo, recipientA, "email")

	bCtx := withTenant(ctx, tenantB)
	if _, err := mRepo.GetByID(bCtx, tenantB, m.ID); err == nil {
		t.Fatal("tenant B should not see tenant A message")
	}
	if _, err := tRepo.GetByID(bCtx, tenantB, tmpl.ID); err == nil {
		t.Fatal("tenant B should not see tenant A template")
	}
	if _, err := sRepo.GetByID(bCtx, tenantB, sub.ID); err == nil {
		t.Fatal("tenant B should not see tenant A subscription")
	}

	mList, _, err := mRepo.List(bCtx, tenantB, ports.MessageFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant B messages: %v", err)
	}
	if len(mList) != 0 {
		t.Fatalf("tenant B should see 0 messages, got %d", len(mList))
	}
}

func TestService_FeatureFlagGatesAccess(t *testing.T) {
	ctx := withTenant(context.Background(), tenantB)
	mRepo, tRepo, sRepo := newAllRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantB, application.FeatureNotifications, false)

	svc := application.NewService(mRepo, tRepo, sRepo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantB, application.PermManage)

	_, err := svc.CreateMessage(ctx, actor, application.CreateMessageRequest{
		RecipientID: recipientA,
		Channel:     "email",
		Subject:     "Hello",
		Body:        "Body",
	})
	if err == nil {
		t.Fatal("expected feature-disabled error")
	}
}

func TestService_FeatureFlagAllowsAccessWhenEnabled(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	mRepo, tRepo, sRepo := newAllRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureNotifications, true)

	svc := application.NewService(mRepo, tRepo, sRepo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantA, application.PermManage)

	m, err := svc.CreateMessage(ctx, actor, application.CreateMessageRequest{
		RecipientID: recipientA,
		Channel:     "email",
		Subject:     "Hello",
		Body:        "Body",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if m.ID == "" {
		t.Fatal("expected message id")
	}
}

func TestService_SendMessageSuccess(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	mRepo, _, sRepo := newAllRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureNotifications, true)
	svc := application.NewService(mRepo, nil, sRepo,
		application.WithFeatureGate(gates),
		application.WithNotifiers(notifier.Registry()),
	)

	actorManage := actorWithPerms(tenantA, application.PermManage)
	actorSend := actorWithPerms(tenantA, application.PermSend)

	_, err := svc.CreateSubscription(ctx, actorManage, application.CreateSubscriptionRequest{
		UserID:    recipientA,
		Channel:   "email",
		IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	m, err := svc.CreateMessage(ctx, actorManage, application.CreateMessageRequest{
		RecipientID: recipientA,
		Channel:     "email",
		Subject:     "Hello",
		Body:        "Body",
	})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}

	sent, err := svc.SendMessage(ctx, actorSend, m.ID)
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if sent.Status != string(domain.MessageStatusSent) {
		t.Fatalf("expected sent status, got %q", sent.Status)
	}
	if sent.SentAt == nil {
		t.Fatal("expected sent_at set")
	}
}

func TestService_SendMessageFailsWithoutSubscription(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	mRepo, _, sRepo := newAllRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureNotifications, true)
	svc := application.NewService(mRepo, nil, sRepo,
		application.WithFeatureGate(gates),
		application.WithNotifiers(notifier.Registry()),
	)

	actorManage := actorWithPerms(tenantA, application.PermManage)
	actorSend := actorWithPerms(tenantA, application.PermSend)

	m, err := svc.CreateMessage(ctx, actorManage, application.CreateMessageRequest{
		RecipientID: recipientA,
		Channel:     "email",
		Subject:     "Hello",
		Body:        "Body",
	})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}

	_, err = svc.SendMessage(ctx, actorSend, m.ID)
	if err == nil {
		t.Fatal("expected send to fail without subscription")
	}

	got, err := mRepo.GetByID(ctx, tenantA, m.ID)
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if got.Status != string(domain.MessageStatusFailed) {
		t.Fatalf("expected failed status, got %q", got.Status)
	}
}

func TestService_SendMessageFailsWhenBodyContainsFail(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	mRepo, _, sRepo := newAllRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureNotifications, true)
	svc := application.NewService(mRepo, nil, sRepo,
		application.WithFeatureGate(gates),
		application.WithNotifiers(notifier.Registry()),
	)

	actorManage := actorWithPerms(tenantA, application.PermManage)
	actorSend := actorWithPerms(tenantA, application.PermSend)

	_, err := svc.CreateSubscription(ctx, actorManage, application.CreateSubscriptionRequest{
		UserID:    recipientA,
		Channel:   "email",
		IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	m, err := svc.CreateMessage(ctx, actorManage, application.CreateMessageRequest{
		RecipientID: recipientA,
		Channel:     "email",
		Subject:     "Hello",
		Body:        "this will fail",
	})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}

	_, err = svc.SendMessage(ctx, actorSend, m.ID)
	if err == nil {
		t.Fatal("expected send to fail")
	}

	got, err := mRepo.GetByID(ctx, tenantA, m.ID)
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if got.Status != string(domain.MessageStatusFailed) {
		t.Fatalf("expected failed status, got %q", got.Status)
	}
	if got.Error == nil {
		t.Fatal("expected error set")
	}
}
