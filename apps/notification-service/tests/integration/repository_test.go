package integration

import (
	"context"
	"testing"

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

const (
	tenantA = "11111111-1111-1111-1111-111111111111"
	tenantB = "22222222-2222-2222-2222-222222222222"

	recipientA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	recipientB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

func newRepos(t *testing.T) (ports.MessageRepository, ports.TemplateRepository, ports.SubscriptionRepository, *testkit.PostgresTestDB) {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewMessageRepository(tdb.DB), postgres.NewTemplateRepository(tdb.DB), postgres.NewSubscriptionRepository(tdb.DB), tdb
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func actorWithPerms(tenantID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantID, Permissions: perms}
}

func mustCreateMessage(t *testing.T, ctx context.Context, repo ports.MessageRepository, tenantID, recipientID, channel, subject, body string) *domain.Message {
	t.Helper()
	m, err := domain.NewMessage(tenantID, recipientID, channel, subject, body, nil, nil, nil)
	if err != nil {
		t.Fatalf("new message: %v", err)
	}
	if err := repo.Create(ctx, tenantID, m); err != nil {
		t.Fatalf("create message: %v", err)
	}
	return m
}

func mustCreateTemplate(t *testing.T, ctx context.Context, repo ports.TemplateRepository, tenantID, name, channel string) *domain.Template {
	t.Helper()
	tmpl, err := domain.NewTemplate(tenantID, name, channel, "Subject", "Body")
	if err != nil {
		t.Fatalf("new template: %v", err)
	}
	if err := repo.Create(ctx, tenantID, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	return tmpl
}

func mustCreateSubscription(t *testing.T, ctx context.Context, repo ports.SubscriptionRepository, tenantID, userID, channel string, enabled bool) *domain.Subscription {
	t.Helper()
	sub, err := domain.NewSubscription(tenantID, userID, channel, enabled)
	if err != nil {
		t.Fatalf("new subscription: %v", err)
	}
	if err := repo.Create(ctx, tenantID, sub); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	return sub
}

func TestMessageRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _, _, _ := newRepos(t)

	m := mustCreateMessage(t, ctx, repo, tenantA, recipientA, "email", "Hello", "Body")

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
	repo, _, _, _ := newRepos(t)

	mustCreateMessage(t, ctx, repo, tenantA, recipientA, "email", "Hello", "Body")
	m2 := mustCreateMessage(t, ctx, repo, tenantA, recipientA, "email", "Hello 2", "Body")

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
	repo, _, _, _ := newRepos(t)

	mustCreateMessage(t, ctx, repo, tenantA, recipientA, "email", "Hello", "Body")
	mustCreateMessage(t, ctx, repo, tenantA, recipientB, "sms", "SMS", "Body")

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
	repo, _, _, _ := newRepos(t)

	m := mustCreateMessage(t, ctx, repo, tenantA, recipientA, "email", "Hello", "Body")
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
	repo, _, _, _ := newRepos(t)

	m := mustCreateMessage(t, ctx, repo, tenantA, recipientA, "email", "Hello", "Body")
	if err := repo.Delete(ctx, tenantA, m.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, tenantA, m.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestTemplateRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	_, repo, _, _ := newRepos(t)

	tmpl := mustCreateTemplate(t, ctx, repo, tenantA, "welcome", "email")

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
	_, repo, _, _ := newRepos(t)

	mustCreateTemplate(t, ctx, repo, tenantA, "welcome", "email")
	mustCreateTemplate(t, ctx, repo, tenantA, "sms_alert", "sms")

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
	_, _, repo, _ := newRepos(t)

	sub := mustCreateSubscription(t, ctx, repo, tenantA, recipientA, "email", true)

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
	_, _, repo, _ := newRepos(t)

	mustCreateSubscription(t, ctx, repo, tenantA, recipientA, "email", true)
	mustCreateSubscription(t, ctx, repo, tenantA, recipientB, "sms", true)

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
	mRepo, tRepo, sRepo, _ := newRepos(t)

	aCtx := withTenant(ctx, tenantA)
	m := mustCreateMessage(t, aCtx, mRepo, tenantA, recipientA, "email", "Hello", "Body")
	tmpl := mustCreateTemplate(t, aCtx, tRepo, tenantA, "welcome", "email")
	sub := mustCreateSubscription(t, aCtx, sRepo, tenantA, recipientA, "email", true)

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
	mRepo, tRepo, sRepo, _ := newRepos(t)

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
	mRepo, tRepo, sRepo, _ := newRepos(t)

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
	mRepo, _, sRepo, _ := newRepos(t)

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
	mRepo, _, sRepo, _ := newRepos(t)

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
	mRepo, _, sRepo, _ := newRepos(t)

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
