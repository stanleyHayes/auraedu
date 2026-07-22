package integration

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/auraedu/campaign-service/internal/adapters/postgres"
	"github.com/auraedu/campaign-service/internal/application"
	"github.com/auraedu/campaign-service/internal/domain"
	"github.com/auraedu/platform/auth"
	platformdb "github.com/auraedu/platform/db"
	"github.com/auraedu/platform/testkit"
)

func TestCampaignPostgresLifecycleAndIsolation(t *testing.T) {
	ctx := context.Background()
	var database *platformdb.DB
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		var e error
		database, e = platformdb.Open(ctx, platformdb.Config{DSN: dsn, Migrations: "../../migrations"})
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(database.Close)
	} else {
		database = testkit.NewPostgres(ctx, t, "../../migrations").DB
	}
	now := time.Now().UTC()
	svc := application.NewService(postgres.NewRepository(database), application.WithClock(func() time.Time { return now }))
	owner := auth.Actor{UserID: "owner", TenantID: "school-one", Permissions: []string{application.PermCreate, application.PermRead, application.PermUpdate}}
	campaign, e := svc.Create(ctx, owner, application.CreateInput{Name: "Open day", Objective: "Qualified applications", Channel: "event", AudienceDefinition: "Prospective students", Budget: 0, Currency: "GHS", StartAt: now.Add(time.Hour), EndAt: now.Add(48 * time.Hour)})
	if e != nil {
		t.Fatal(e)
	}
	campaign, e = svc.Submit(ctx, owner, campaign.ID)
	if e != nil {
		t.Fatal(e)
	}
	reviewer := auth.Actor{UserID: "reviewer", TenantID: "school-one", Permissions: []string{application.PermApprove}}
	campaign, e = svc.Approve(ctx, reviewer, campaign.ID, "Verified audience and dates")
	if e != nil || campaign.Status != domain.StatusApproved {
		t.Fatalf("campaign=%+v err=%v", campaign, e)
	}
	outbox, e := postgres.NewRepository(database).ClaimPending(ctx, 10)
	if e != nil || len(outbox) != 2 {
		t.Fatalf("transactional outbox=%+v err=%v", outbox, e)
	}
	if outbox[0].TenantID != "school-one" || outbox[0].EventType != "campaign.status_changed.v1" {
		t.Fatalf("unexpected outbox event: %+v", outbox[0])
	}
	other := auth.Actor{UserID: "other", TenantID: "school-two", Permissions: []string{application.PermRead}}
	if _, e = svc.Get(ctx, other, campaign.ID); !errors.Is(e, domain.ErrNotFound) {
		t.Fatalf("cross tenant get=%v", e)
	}
}
