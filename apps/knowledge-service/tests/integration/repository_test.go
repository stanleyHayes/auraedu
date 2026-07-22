package integration

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/auraedu/knowledge-service/internal/adapters/postgres"
	"github.com/auraedu/knowledge-service/internal/application"
	"github.com/auraedu/knowledge-service/internal/domain"
	"github.com/auraedu/platform/auth"
	platformdb "github.com/auraedu/platform/db"
	"github.com/auraedu/platform/testkit"
)

func TestPostgresKnowledgeReviewRetrievalAndTenantIsolation(t *testing.T) {
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
	now := time.Now().UTC().Truncate(time.Second)
	repo := postgres.NewRepository(database)
	svc := application.NewService(repo, application.WithClock(func() time.Time { return now }))
	manager := auth.Actor{UserID: "manager", TenantID: "school-one", Permissions: []string{application.PermManage}}
	reviewer := auth.Actor{UserID: "reviewer", TenantID: "school-one", Permissions: []string{application.PermApprove}}
	source, err := svc.Create(ctx, manager, application.CreateInput{SourceType: "fees", Title: "Application fees",
		Owner: "Admissions", Content: "The verified application fee is GHS 250 and is paid in the applicant portal.",
		Confidentiality: "public", EffectiveAt: now.Add(-time.Hour)})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Approve(ctx, reviewer, source.ID, "Checked against signed schedule"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	results, err := svc.SearchApproved(ctx, "school-one", "verified application fee", "en-GH", 5, now)
	if err != nil || len(results) != 1 || results[0].SourceID != source.ID {
		t.Fatalf("search: results=%+v err=%v", results, err)
	}
	other, err := svc.SearchApproved(ctx, "school-two", "verified application fee", "en", 5, now)
	if err != nil || len(other) != 0 {
		t.Fatalf("cross-tenant search: results=%+v err=%v", other, err)
	}
	french, err := svc.Create(ctx, manager, application.CreateInput{SourceType: "fees", Title: "Frais de candidature",
		Owner: "Admissions", Content: "Les frais de candidature vérifiés sont de deux cent cinquante cedis.",
		Confidentiality: "public", Locale: "fr-GH", EffectiveAt: now.Add(-time.Hour)})
	if err != nil {
		t.Fatalf("create French source: %v", err)
	}
	if _, err := svc.Approve(ctx, reviewer, french.ID, "Version française vérifiée"); err != nil {
		t.Fatalf("approve French source: %v", err)
	}
	frenchResults, err := svc.SearchApproved(ctx, "school-one", "frais candidature", "fr", 5, now)
	if err != nil || len(frenchResults) != 1 || frenchResults[0].SourceID != french.ID || frenchResults[0].Locale != "fr-GH" {
		t.Fatalf("French search: results=%+v err=%v", frenchResults, err)
	}
	wrongLanguage, err := svc.SearchApproved(ctx, "school-one", "frais candidature", "en", 5, now)
	if err != nil || len(wrongLanguage) != 0 {
		t.Fatalf("cross-language search leaked: results=%+v err=%v", wrongLanguage, err)
	}
	events, err := repo.ClaimPending(context.Background(), 10)
	if err != nil {
		t.Fatalf("claim approval events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected one durable event per approval, got %+v", events)
	}
	for _, event := range events {
		if event.ID == "" || event.TenantID != "school-one" || event.EventType != "knowledge.source_approved.v1" {
			t.Fatalf("invalid approval event identity: %+v", event)
		}
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode approval event: %v", err)
		}
		for _, forbidden := range []string{"content", "owner", "approved_by", "review_note"} {
			if _, exposed := payload[forbidden]; exposed {
				t.Fatalf("approval event leaked %s: %+v", forbidden, payload)
			}
		}
	}
}

func TestPostgresKnowledgeApprovalRollsBackWithoutOutbox(t *testing.T) {
	ctx := context.Background()
	database := testkit.NewPostgres(ctx, t, "../../migrations").DB
	repo := postgres.NewRepository(database)
	now := time.Now().UTC().Truncate(time.Second)
	svc := application.NewService(repo, application.WithClock(func() time.Time { return now }))
	manager := auth.Actor{UserID: "manager", TenantID: "school-one", Permissions: []string{application.PermManage}}
	reviewer := auth.Actor{UserID: "reviewer", TenantID: "school-one", Permissions: []string{application.PermApprove}}
	source, err := svc.Create(ctx, manager, application.CreateInput{
		SourceType: "policy", Title: "Admissions policy", Owner: "Registry",
		Content:         "Applicants must submit every required official document before the review deadline.",
		Confidentiality: "public", EffectiveAt: now,
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	if _, err := database.Pool().Exec(ctx, `DROP TABLE knowledge_outbox`); err != nil {
		t.Fatalf("remove outbox: %v", err)
	}
	if _, err := svc.Approve(ctx, reviewer, source.ID, "Verified against the signed policy"); err == nil {
		t.Fatal("approval must fail when its lifecycle event cannot be committed")
	}
	stored, err := repo.Get(ctx, "school-one", source.ID)
	if err != nil {
		t.Fatalf("get rolled-back source: %v", err)
	}
	if stored.Status != domain.StatusDraft || stored.ApprovedAt != nil || stored.ApprovedBy != nil {
		t.Fatalf("approval committed without its event: %+v", stored)
	}
}
