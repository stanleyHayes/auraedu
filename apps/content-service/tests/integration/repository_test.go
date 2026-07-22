package integration

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/auraedu/content-service/internal/adapters/postgres"
	"github.com/auraedu/content-service/internal/application"
	"github.com/auraedu/content-service/internal/domain"
	"github.com/auraedu/content-service/internal/ports"
	"github.com/auraedu/platform/auth"
	platformdb "github.com/auraedu/platform/db"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

type generator struct{}

func (generator) Generate(context.Context, ports.GenerateInput) (ports.GenerateOutput, error) {
	return ports.GenerateOutput{Content: "Meet our teachers on 30 August. Terms apply", Generator: "integration:model"}, nil
}

func TestContentPostgresLifecycleVersionsOutboxAndIsolation(t *testing.T) {
	ctx := context.Background()
	var database *platformdb.DB
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		var err error
		database, err = platformdb.Open(ctx, platformdb.Config{DSN: dsn, Migrations: "../../migrations"})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(database.Close)
	} else {
		database = testkit.NewPostgres(ctx, t, "../../migrations").DB
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	repository := postgres.NewRepository(database)
	service := application.NewService(repository, generator{}, application.WithClock(func() time.Time { return now }))
	tenantID := "content-" + uuid.NewString()
	author := auth.Actor{UserID: "author", TenantID: tenantID, Permissions: []string{application.PermGenerate}}
	reviewer := auth.Actor{UserID: "reviewer", TenantID: tenantID, Permissions: []string{application.PermReview}}
	if _, err := service.UpsertBrandProfile(ctx, reviewer, application.BrandProfileInput{ToneOfVoice: "Warm, factual and encouraging", Locale: "en-GH", RequiredDisclaimers: []string{"Terms apply"}, ProhibitedClaims: []string{"guaranteed admission"}}); err != nil {
		t.Fatal(err)
	}
	draft, err := service.Generate(ctx, author, application.GenerateInput{IdempotencyKey: "integration-content-0001", ContentType: "social_post", Title: "Open day", Brief: "Create an invitation for our next open day.", Audience: "Prospective families", Locale: "en-GH", KeyMessages: []string{"Meet our teachers"}, Facts: []domain.Fact{{Label: "Date", Value: "30 August"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Submit(ctx, author, draft.ID, 1); err != nil {
		t.Fatal(err)
	}
	approved, err := service.Approve(ctx, reviewer, draft.ID, "Facts and policy verified", 1)
	if err != nil || approved.Status != domain.StatusApproved {
		t.Fatalf("approved=%#v err=%v", approved, err)
	}
	loaded, versions, err := service.Get(ctx, reviewer, draft.ID)
	if err != nil || loaded.Status != domain.StatusApproved || len(versions) != 1 || versions[0].Content != draft.Content {
		t.Fatalf("loaded=%#v versions=%#v err=%v", loaded, versions, err)
	}
	outbox, err := repository.ClaimPending(ctx, 10)
	if err != nil || len(outbox) != 3 {
		t.Fatalf("outbox=%#v err=%v", outbox, err)
	}
	for _, event := range outbox {
		if event.TenantID != author.TenantID || string(event.Payload) == "" {
			t.Fatalf("unsafe event=%#v", event)
		}
	}
	other := auth.Actor{UserID: "other", TenantID: "content-school-two", Permissions: []string{application.PermGenerate}}
	if _, _, err := service.Get(ctx, other, draft.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant lookup must be not found, got %v", err)
	}
}

func TestBrandProfileOptimisticConcurrency(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required for the focused optimistic-concurrency proof")
	}
	database, err := platformdb.Open(ctx, platformdb.Config{DSN: dsn, Migrations: "../../migrations"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(database.Close)
	service := application.NewService(postgres.NewRepository(database), generator{})
	reviewer := auth.Actor{UserID: "reviewer", TenantID: "content-" + uuid.NewString(), Permissions: []string{application.PermReview}}
	if _, err := service.UpsertBrandProfile(ctx, reviewer, application.BrandProfileInput{ToneOfVoice: "Clear and warm", Locale: "en-GH"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpsertBrandProfile(ctx, reviewer, application.BrandProfileInput{ToneOfVoice: "Stale update", Locale: "en-GH", ExpectedVersion: 0}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale profile update must conflict, got %v", err)
	}
}
