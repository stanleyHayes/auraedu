// Package workercmd provisions the first school administrator after a reviewed
// onboarding request creates its tenant.
package workercmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/auraedu/identity-service/internal/adapters/events"
	notificationadapter "github.com/auraedu/identity-service/internal/adapters/notification"
	"github.com/auraedu/identity-service/internal/adapters/postgres"
	tenantadapter "github.com/auraedu/identity-service/internal/adapters/tenant"
	"github.com/auraedu/identity-service/internal/application"
	identitydb "github.com/auraedu/identity-service/internal/db"
	"github.com/auraedu/identity-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/platform/tenancy"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

const service = "identity-service-worker"

const (
	defaultAuthCleanupInterval         = time.Hour
	defaultRefreshRetentionAfterExpiry = 24 * time.Hour
	defaultPasswordResetRetention      = 30 * 24 * time.Hour
	defaultInviteRetention             = 90 * 24 * time.Hour
	defaultPublishedOutboxRetention    = 30 * 24 * time.Hour
	defaultAuthCleanupBatchSize        = 1000
)

type authRetentionPolicy struct {
	CleanupInterval          time.Duration
	RefreshAfterExpiry       time.Duration
	PasswordResetRetention   time.Duration
	InviteRetention          time.Duration
	PublishedOutboxRetention time.Duration
	BatchSize                int
}

func run(log *slog.Logger) error {
	if err := config.RequireProductionEnv("INTERNAL_SERVICE_TOKEN"); err != nil {
		return err
	}
	retention, err := loadAuthRetentionPolicy()
	if err != nil {
		return err
	}
	ctx := context.Background()
	shutdownTelemetry, err := observ.InitTracing(service, config.Getenv("GIT_SHA", "dev"))
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.Error("flush identity worker telemetry", "err", err)
		}
	}()
	dsn, err := config.MustGetenv("DATABASE_URL")
	if err != nil {
		return err
	}
	pool, err := identitydb.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := identitydb.Migrate(ctx, pool); err != nil {
		return err
	}

	natsURL, err := config.MustGetenv("NATS_URL")
	if err != nil {
		return err
	}
	nc, js, err := connectEventBus(natsURL)
	if err != nil {
		return err
	}
	defer nc.Close()
	signingKey, err := config.MustGetenv("JWT_SIGNING_KEY")
	if err != nil {
		return err
	}

	publisher := events.NewPublisher(eventbus.NewPublisher(js))
	repository := postgres.NewRepository(pool)
	svc := application.NewService(repository, nil, publisher,
		[]byte(signingKey), 15*time.Minute, 7*24*time.Hour,
		application.WithTransactionalNotifier(notificationadapter.NewClient(
			config.Getenv("SERVICE_NOTIFICATION_URL", "http://notification-service:8099"),
			config.Getenv("INTERNAL_SERVICE_TOKEN", ""),
		)),
	)
	resolver := tenantadapter.NewClient(
		config.Getenv("SERVICE_TENANT_URL", "http://tenant-service:8082"),
		config.Getenv("INTERNAL_SERVICE_TOKEN", ""),
	)

	metrics := observ.NewWorkerMetrics(service, "onboarding-approved", "outbox-batch", "outbox-publish", "auth-cleanup")
	cleanupStarted := time.Now()
	cleanupResult, err := runAuthCleanup(ctx, repository, retention, cleanupStarted)
	metrics.Observe(ctx, "auth-cleanup", cleanupStarted, err)
	if err != nil {
		return fmt.Errorf("identity worker: initial auth cleanup: %w", err)
	}
	logAuthCleanup(log, cleanupResult)
	subscription, err := eventbus.Subscribe(
		js, "AURA", "identity-onboarding-approved", "tenant.onboarding_approved.v1",
		func(handlerCtx context.Context, event tenancy.CloudEvent) error {
			started := time.Now()
			err := handleOnboardingApproved(handlerCtx, log, pool, resolver, svc, event)
			metrics.Observe(handlerCtx, "onboarding-approved", started, err)
			return err
		}, nil,
	)
	if err != nil {
		return err
	}
	defer func() {
		if unsubscribeErr := subscription.Unsubscribe(); unsubscribeErr != nil {
			log.Error("unsubscribe identity onboarding worker", "err", unsubscribeErr)
		}
	}()

	log.Info(service + " started")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	outboxTicker := time.NewTicker(time.Second)
	defer outboxTicker.Stop()
	cleanupTicker := time.NewTicker(retention.CleanupInterval)
	defer cleanupTicker.Stop()
	for {
		select {
		case <-stop:
			return nil
		case <-outboxTicker.C:
			started := time.Now()
			err := dispatchOutbox(ctx, repository, publisher, metrics)
			metrics.Observe(ctx, "outbox-batch", started, err)
			if err != nil {
				log.Error("identity outbox dispatch failed", "err", err)
			}
		case now := <-cleanupTicker.C:
			started := time.Now()
			result, err := runAuthCleanup(ctx, repository, retention, now)
			metrics.Observe(ctx, "auth-cleanup", started, err)
			if err != nil {
				log.Error("identity auth cleanup failed", "err", err)
				continue
			}
			logAuthCleanup(log, result)
		}
	}
}

func connectEventBus(natsURL string) (*nats.Conn, eventbus.JetStreamContext, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, nil, fmt.Errorf("identity worker: connect NATS: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, nil, err
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		nc.Close()
		return nil, nil, err
	}
	return nc, js, nil
}

func loadAuthRetentionPolicy() (authRetentionPolicy, error) {
	values := []struct {
		name        string
		fallback    time.Duration
		destination *time.Duration
	}{
		{name: "AUTH_CLEANUP_INTERVAL", fallback: defaultAuthCleanupInterval},
		{name: "AUTH_REFRESH_RETENTION_AFTER_EXPIRY", fallback: defaultRefreshRetentionAfterExpiry},
		{name: "AUTH_PASSWORD_RESET_RETENTION", fallback: defaultPasswordResetRetention},
		{name: "AUTH_INVITE_RETENTION", fallback: defaultInviteRetention},
		{name: "AUTH_PUBLISHED_OUTBOX_RETENTION", fallback: defaultPublishedOutboxRetention},
	}
	policy := authRetentionPolicy{}
	values[0].destination = &policy.CleanupInterval
	values[1].destination = &policy.RefreshAfterExpiry
	values[2].destination = &policy.PasswordResetRetention
	values[3].destination = &policy.InviteRetention
	values[4].destination = &policy.PublishedOutboxRetention
	for _, value := range values {
		duration := value.fallback
		if raw, ok := os.LookupEnv(value.name); ok {
			parsed, err := time.ParseDuration(raw)
			if err != nil {
				return authRetentionPolicy{}, fmt.Errorf("identity worker: parse %s: %w", value.name, err)
			}
			duration = parsed
		}
		if duration <= 0 {
			return authRetentionPolicy{}, fmt.Errorf("identity worker: %s must be greater than zero", value.name)
		}
		*value.destination = duration
	}
	policy.BatchSize = defaultAuthCleanupBatchSize
	if raw, ok := os.LookupEnv("AUTH_CLEANUP_BATCH_SIZE"); ok {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return authRetentionPolicy{}, fmt.Errorf("identity worker: parse AUTH_CLEANUP_BATCH_SIZE: %w", err)
		}
		policy.BatchSize = parsed
	}
	if policy.BatchSize <= 0 || policy.BatchSize > 10_000 {
		return authRetentionPolicy{}, fmt.Errorf("identity worker: AUTH_CLEANUP_BATCH_SIZE must be between 1 and 10000")
	}
	return policy, nil
}

func (p authRetentionPolicy) cutoffs(now time.Time) ports.AuthRetentionCutoffs {
	return ports.AuthRetentionCutoffs{
		RefreshFamiliesBefore: now.Add(-p.RefreshAfterExpiry),
		PasswordResetsBefore:  now.Add(-p.PasswordResetRetention),
		InvitesBefore:         now.Add(-p.InviteRetention),
		PublishedOutboxBefore: now.Add(-p.PublishedOutboxRetention),
		BatchSize:             p.BatchSize,
	}
}

func runAuthCleanup(ctx context.Context, repo ports.AuthCleanupRepository, policy authRetentionPolicy, now time.Time) (ports.AuthCleanupResult, error) {
	return repo.CleanupAuthArtifacts(ctx, policy.cutoffs(now))
}

func logAuthCleanup(log *slog.Logger, result ports.AuthCleanupResult) {
	if result.RefreshTokens == 0 && result.PasswordResets == 0 && result.Invites == 0 && result.OutboxEvents == 0 {
		return
	}
	log.Info("identity auth cleanup completed",
		"refresh_tokens", result.RefreshTokens,
		"password_resets", result.PasswordResets,
		"invites", result.Invites,
		"outbox_events", result.OutboxEvents,
	)
}

type outboxRepository interface {
	ClaimPending(context.Context, int) ([]ports.OutboxEvent, error)
	MarkPublished(context.Context, string) error
	MarkFailed(context.Context, string, string) error
}

func dispatchOutbox(ctx context.Context, repo outboxRepository, publisher ports.EventPublisher, metrics *observ.WorkerMetrics) error {
	events, err := repo.ClaimPending(ctx, 25)
	if err != nil {
		return err
	}
	for _, item := range events {
		started := time.Now()
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkFailed(ctx, item.ID, "invalid outbox payload"); markErr != nil {
				return markErr
			}
			continue
		}
		event := ports.Event{
			SpecVersion: "1.0", Type: item.EventType, Source: "identity-service",
			ID: item.ID, Time: item.CreatedAt, TenantID: item.TenantID,
			DataContentType: "application/json", Data: payload,
		}
		if err := publisher.Publish(ctx, event); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkFailed(ctx, item.ID, err.Error()); markErr != nil {
				return markErr
			}
			continue
		}
		if err := repo.MarkPublished(ctx, item.ID); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			return err
		}
		metrics.Observe(ctx, "outbox-publish", started, nil)
	}
	return nil
}

func handleOnboardingApproved(
	ctx context.Context,
	log *slog.Logger,
	pool *pgxpool.Pool,
	resolver *tenantadapter.Client,
	svc *application.Service,
	event tenancy.CloudEvent,
) error {
	var payload struct {
		RequestID  string `json:"request_id"`
		TenantCode string `json:"tenant_code"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil || payload.RequestID == "" || payload.TenantCode == "" {
		return fmt.Errorf("identity onboarding: invalid approval event")
	}
	claimed, err := claimEvent(ctx, pool, event.ID, event.Type, payload.TenantCode)
	if err != nil || !claimed {
		return err
	}
	release := true
	defer func() {
		if release {
			if err := releaseEvent(context.Background(), pool, event.ID, payload.TenantCode); err != nil {
				log.Error("failed to release onboarding event claim", "event_id", event.ID, "err", err)
			}
		}
	}()

	administrator, err := resolver.ResolveOnboardingAdministrator(ctx, payload.RequestID)
	if err != nil {
		return err
	}
	if administrator.TenantCode != payload.TenantCode || administrator.TenantCode != event.TenantID {
		return fmt.Errorf("identity onboarding: tenant mismatch")
	}
	actor := auth.Actor{
		Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true,
		Permissions: []string{application.PermUsersCreate},
	}
	_, err = svc.InviteUser(ctx, actor, application.InviteInput{
		TenantID: administrator.TenantCode, Email: administrator.Email,
		Role: "school_admin", Permissions: schoolAdminPermissions(),
	})
	if err != nil {
		return err
	}
	release = false
	log.Info("created onboarding administrator invite", "tenant_id", administrator.TenantCode, "request_id", payload.RequestID)
	return nil
}

func claimEvent(ctx context.Context, pool *pgxpool.Pool, eventID, eventType, tenantID string) (bool, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if err := identitydb.SetTenantContext(ctx, tx, tenantID, false); err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO identity_processed_events (event_id, event_type, tenant_id)
		VALUES ($1, $2, $3) ON CONFLICT (event_id) DO NOTHING
	`, eventID, eventType, tenantID)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func releaseEvent(ctx context.Context, pool *pgxpool.Pool, eventID, tenantID string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := identitydb.SetTenantContext(ctx, tx, tenantID, false); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM identity_processed_events WHERE event_id = $1`, eventID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func schoolAdminPermissions() []string {
	return []string{
		"features.manage", "users.read", "users.create", "users.update", "roles.assign",
		"students.read", "students.create", "students.update", "students.delete",
		"staff.read", "staff.create", "staff.update", "academic.read", "academic.manage",
		"attendance.read", "attendance.mark", "assessments.read", "assessments.record_scores", "assessments.manage",
		"reports.read", "reports.publish", "fees.read", "fees.manage", "payments.read", "payments.initiate", "payments.configure",
		"notifications.read", "notifications.send", "notifications.manage", "website.read", "website.manage",
		"files.read", "files.upload", "files.update", "files.delete", "analytics.view", "billing.read", "billing.manage", "audit.read",
		"cbt.read", "cbt.author", "cbt.take", "cbt.grade",
		"ai.view_recommendations", "ai.approve_recommendations", "ai.view_predictions",
		"ai.approve_predictions", "ai.view_guidance", "ai.approve_guidance",
		"crm.lead.read", "crm.lead.create", "crm.lead.update", "crm.lead.assign", "crm.lead.export", "crm.interaction.create",
		"knowledge.read", "knowledge.manage", "knowledge.approve", "feedback.review", "analytics.executive.read",
		"campaign.read", "campaign.create", "campaign.update", "campaign.approve", "campaign.publish", "campaign.budget.approve",
		"admissions.catalogue.manage", "admissions.application.read", "admissions.application.review", "admissions.offer.issue",
		"intelligence.read", "intelligence.manage", "intelligence.review",
	}
}

// Run starts the identity-service background worker. It is invoked by the service CLI.
func Run() error {
	log := observ.DefaultLogger()
	slog.SetDefault(log)
	return run(log)
}
