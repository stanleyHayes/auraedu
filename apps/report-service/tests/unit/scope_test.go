package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/report-service/internal/application"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
)

type reportLearnerResolver struct{ ids []string }

func (r reportLearnerResolver) Resolve(context.Context, string, string, string) ([]string, error) {
	return r.ids, nil
}

func reportContext() context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantA})
}

func TestParentReportCardsAreOwnedAndPublishedOnly(t *testing.T) {
	repo := newFakeRepo()
	own, err := domain.NewReportCard(tenantA, studentA, ay1, template1)
	if err != nil {
		t.Fatal(err)
	}
	own.Status = string(domain.ReportCardStatusPublished)
	other, err := domain.NewReportCard(tenantA, "student-other", ay1, template1)
	if err != nil {
		t.Fatal(err)
	}
	other.Status = string(domain.ReportCardStatusPublished)
	draft, err := domain.NewReportCard(tenantA, studentA, ay1, template1)
	if err != nil {
		t.Fatal(err)
	}
	for _, card := range []*domain.ReportCard{own, other, draft} {
		if err := repo.CreateReportCard(reportContext(), tenantA, card); err != nil {
			t.Fatal(err)
		}
	}
	svc := application.NewService(repo,
		application.WithFeatureGate(enabledGates(tenantA)),
		application.WithLearnerScopeResolver(reportLearnerResolver{ids: []string{studentA}}),
	)
	actor := auth.Actor{UserID: "parent-1", TenantID: tenantA, Role: "parent", Permissions: []string{application.PermRead}}

	cards, _, err := svc.ListReportCards(reportContext(), actor, ports.ReportCardListFilter{})
	if err != nil {
		t.Fatalf("list report cards: %v", err)
	}
	if len(cards) != 1 || cards[0].ID != own.ID {
		t.Fatalf("expected only owned published card, got %+v", cards)
	}
	if _, err := svc.GetReportCard(reportContext(), actor, other.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected other learner card hidden, got %v", err)
	}
	if _, err := svc.GetReportCard(reportContext(), actor, draft.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected draft card hidden, got %v", err)
	}
}

func TestParentReportScopeFailsClosedWithoutResolver(t *testing.T) {
	svc := application.NewService(newFakeRepo(), application.WithFeatureGate(enabledGates(tenantA)))
	actor := auth.Actor{UserID: "parent-1", TenantID: tenantA, Role: "parent", Permissions: []string{application.PermRead}}
	if _, _, err := svc.ListReportCards(reportContext(), actor, ports.ReportCardListFilter{}); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("expected unavailable without learner resolver, got %v", err)
	}
}

func TestTeacherReportCardsAreAssignedLearnersOnly(t *testing.T) {
	repo := newFakeRepo()
	assigned, err := domain.NewReportCard(tenantA, studentA, ay1, template1)
	if err != nil {
		t.Fatal(err)
	}
	other, err := domain.NewReportCard(tenantA, "student-other", ay1, template1)
	if err != nil {
		t.Fatal(err)
	}
	for _, card := range []*domain.ReportCard{assigned, other} {
		if err := repo.CreateReportCard(reportContext(), tenantA, card); err != nil {
			t.Fatal(err)
		}
	}
	svc := application.NewService(repo,
		application.WithFeatureGate(enabledGates(tenantA)),
		application.WithLearnerScopeResolver(reportLearnerResolver{ids: []string{studentA}}),
	)
	actor := auth.Actor{UserID: "teacher-1", TenantID: tenantA, Role: "teacher", Permissions: []string{application.PermRead}}

	cards, _, err := svc.ListReportCards(reportContext(), actor, ports.ReportCardListFilter{})
	if err != nil {
		t.Fatalf("list report cards: %v", err)
	}
	if len(cards) != 1 || cards[0].ID != assigned.ID {
		t.Fatalf("expected only assigned learner draft, got %+v", cards)
	}
	if _, err := svc.GetReportCard(reportContext(), actor, other.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected unassigned learner card hidden, got %v", err)
	}
	if _, err := svc.GetReportCard(reportContext(), actor, assigned.ID); err != nil {
		t.Fatalf("assigned draft should remain visible to teacher: %v", err)
	}
}

func TestTeacherReportScopeFailsClosedWithoutResolver(t *testing.T) {
	svc := application.NewService(newFakeRepo(), application.WithFeatureGate(enabledGates(tenantA)))
	actor := auth.Actor{UserID: "teacher-1", TenantID: tenantA, Role: "teacher", Permissions: []string{application.PermRead}}
	if _, _, err := svc.ListReportCards(reportContext(), actor, ports.ReportCardListFilter{}); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("expected unavailable without teacher learner scope, got %v", err)
	}
}

func TestTranscriptUsesOnlyOwnedPublishedAcademicEvidence(t *testing.T) {
	repo := newFakeRepo()
	card, err := domain.NewReportCard(tenantA, studentA, ay1, template1)
	if err != nil {
		t.Fatal(err)
	}
	card.Status = string(domain.ReportCardStatusPublished)
	if err := repo.CreateReportCard(reportContext(), tenantA, card); err != nil {
		t.Fatal(err)
	}
	maximum := 100.0
	score, err := domain.NewScoreEntry(tenantA, card.ID, studentA, "math", "assessment-1", "event-1", 84, &maximum)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertScoreEntry(reportContext(), tenantA, score); err != nil {
		t.Fatal(err)
	}
	svc := application.NewService(repo,
		application.WithFeatureGate(enabledGates(tenantA)),
		application.WithLearnerScopeResolver(reportLearnerResolver{ids: []string{studentA}}),
	)
	actor := auth.Actor{UserID: "parent-1", TenantID: tenantA, Role: "parent", Permissions: []string{application.PermRead}}
	transcript, err := svc.GetTranscript(reportContext(), actor, studentA)
	if err != nil {
		t.Fatalf("get transcript: %v", err)
	}
	if len(transcript.Entries) != 1 || len(transcript.Entries[0].Scores) != 1 || transcript.Entries[0].Scores[0].Score != 84 {
		t.Fatalf("unexpected transcript: %+v", transcript)
	}
	if _, err := svc.GetTranscript(reportContext(), actor, "student-other"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected other learner transcript hidden, got %v", err)
	}
}

func TestReportMutationsRequirePublishPermission(t *testing.T) {
	svc := application.NewService(newFakeRepo(), application.WithFeatureGate(enabledGates(tenantA)))
	actor := auth.Actor{UserID: "reader-1", TenantID: tenantA, Permissions: []string{application.PermRead}}
	name := "Changed"
	tests := []struct {
		name string
		run  func() error
	}{
		{"create template", func() error {
			_, err := svc.CreateReportTemplate(reportContext(), actor, application.CreateReportTemplateRequest{Name: "Term", AcademicYearID: ay1, BodyTemplate: "body"})
			return err
		}},
		{"update template", func() error {
			_, err := svc.UpdateReportTemplate(reportContext(), actor, "template-1", application.UpdateReportTemplateRequest{Name: &name})
			return err
		}},
		{"delete template", func() error { return svc.DeleteReportTemplate(reportContext(), actor, "template-1") }},
		{"create report card", func() error {
			_, err := svc.CreateReportCard(reportContext(), actor, application.CreateReportCardRequest{StudentID: studentA, AcademicYearID: ay1, TemplateID: template1})
			return err
		}},
		{"update report card", func() error {
			_, err := svc.UpdateReportCard(reportContext(), actor, "card-1", application.UpdateReportCardRequest{})
			return err
		}},
		{"delete report card", func() error { return svc.DeleteReportCard(reportContext(), actor, "card-1") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, domain.ErrForbidden) {
				t.Fatalf("expected forbidden, got %v", err)
			}
		})
	}
}
