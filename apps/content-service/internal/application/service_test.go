package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/content-service/internal/adapters/memory"
	"github.com/auraedu/content-service/internal/domain"
	"github.com/auraedu/content-service/internal/ports"
	"github.com/auraedu/platform/auth"
)

type fakeGenerator struct{ calls int }

func (g *fakeGenerator) Generate(_ context.Context, _ ports.GenerateInput) (ports.GenerateOutput, error) {
	g.calls++
	return ports.GenerateOutput{Content: "Meet our teachers on 30 August. Terms apply", Generator: "test:model"}, nil
}

func actor(id string, permissions ...string) auth.Actor {
	return auth.Actor{TenantID: "school-a", UserID: id, Permissions: permissions}
}

func createProfile(t *testing.T, svc *Service, reviewer auth.Actor) {
	t.Helper()
	_, err := svc.UpsertBrandProfile(context.Background(), reviewer, BrandProfileInput{ToneOfVoice: "Warm, factual and encouraging", Locale: "en-GH", RequiredDisclaimers: []string{"Terms apply"}, ProhibitedClaims: []string{"guaranteed admission"}})
	if err != nil {
		t.Fatal(err)
	}
}

func generate(t *testing.T, svc *Service, author auth.Actor, key string) domain.Draft {
	t.Helper()
	draft, err := svc.Generate(context.Background(), author, GenerateInput{IdempotencyKey: key, ContentType: "social_post", Title: "Open day", Brief: "Create an invitation for our next open day.", Audience: "Prospective families", Locale: "en-GH", KeyMessages: []string{"Meet our teachers"}, Facts: []domain.Fact{{Label: "Date", Value: "30 August"}}})
	if err != nil {
		t.Fatal(err)
	}
	return draft
}

func TestGenerationIsReplaySafeAndFourEyesReviewed(t *testing.T) {
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	repo, generator := memory.NewRepository(), &fakeGenerator{}
	svc := NewService(repo, generator, WithClock(func() time.Time { return now }))
	author, reviewer := actor("author", PermGenerate), actor("reviewer", PermReview)
	createProfile(t, svc, reviewer)
	draft := generate(t, svc, author, "content-request-0001")
	replay := generate(t, svc, author, "content-request-0001")
	if draft.ID != replay.ID || generator.calls != 1 {
		t.Fatalf("expected one provider call and same draft, got calls=%d ids=%s/%s", generator.calls, draft.ID, replay.ID)
	}
	if _, err := svc.Submit(context.Background(), author, draft.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Approve(context.Background(), author, draft.ID, "Self review", 1); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("author lacks review permission, got %v", err)
	}
	approved, err := svc.Approve(context.Background(), reviewer, draft.ID, "Facts and policy verified", 1)
	if err != nil || approved.Status != domain.StatusApproved {
		t.Fatalf("expected approval, got %#v %v", approved, err)
	}
}

func TestChangedIdempotencyReplayConflicts(t *testing.T) {
	repo, generator := memory.NewRepository(), &fakeGenerator{}
	svc := NewService(repo, generator)
	author, reviewer := actor("author", PermGenerate), actor("reviewer", PermReview)
	createProfile(t, svc, reviewer)
	_ = generate(t, svc, author, "content-request-0002")
	_, err := svc.Generate(context.Background(), author, GenerateInput{IdempotencyKey: "content-request-0002", ContentType: "email", Title: "Different", Brief: "This request has materially different content.", Audience: "Families", Locale: "en-GH", KeyMessages: []string{"Different"}, Facts: []domain.Fact{{Label: "Fact", Value: "Different"}}})
	if !errors.Is(err, domain.ErrConflict) || generator.calls != 1 {
		t.Fatalf("changed replay must conflict before provider call, got calls=%d err=%v", generator.calls, err)
	}
}

func TestExactReplayDoesNotDependOnMutableBrandProfile(t *testing.T) {
	repo, generator := memory.NewRepository(), &fakeGenerator{}
	svc := NewService(repo, generator)
	author, reviewer := actor("author", PermGenerate), actor("reviewer", PermReview)
	createProfile(t, svc, reviewer)
	draft := generate(t, svc, author, "content-request-0004")
	if _, err := svc.UpsertBrandProfile(context.Background(), reviewer, BrandProfileInput{
		ToneOfVoice: "Clear, calm and factual", Locale: "en-GH", ExpectedVersion: 1,
		RequiredDisclaimers: []string{"Terms apply"},
	}); err != nil {
		t.Fatal(err)
	}
	replay := generate(t, svc, author, "content-request-0004")
	if replay.ID != draft.ID || generator.calls != 1 {
		t.Fatalf("exact replay must return the original result, got calls=%d ids=%s/%s", generator.calls, draft.ID, replay.ID)
	}
}

func TestListRejectsUnknownFilters(t *testing.T) {
	svc := NewService(memory.NewRepository(), &fakeGenerator{})
	author := actor("author", PermGenerate)
	for name, filter := range map[string]ports.ListFilter{
		"status":       {Status: "published"},
		"content type": {ContentType: "press_release"},
		"campaign":     {CampaignID: "not-a-uuid"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.List(context.Background(), author, filter); !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestTenantIsolationInMemoryRepository(t *testing.T) {
	repo, generator := memory.NewRepository(), &fakeGenerator{}
	svc := NewService(repo, generator)
	author, reviewer := actor("author", PermGenerate), actor("reviewer", PermReview)
	createProfile(t, svc, reviewer)
	draft := generate(t, svc, author, "content-request-0003")
	other := actor("other", PermGenerate)
	other.TenantID = "school-b"
	if _, _, err := svc.Get(context.Background(), other, draft.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant lookup must not disclose draft, got %v", err)
	}
}
