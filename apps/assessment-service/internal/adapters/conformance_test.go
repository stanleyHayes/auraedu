// Package adapters_test runs one contract against every assessment repository driver.
//
// Swapping persistence is only safe if both adapters behave identically, so the
// assertions live here once and each driver runs them.
package adapters_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mongoadapter "github.com/auraedu/assessment-service/internal/adapters/mongo"
	pgadapter "github.com/auraedu/assessment-service/internal/adapters/postgres"
	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/assessment-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

type assessmentRepo interface {
	ports.Repository
	ports.LifecycleRepository
	ports.OutboxRepository
}

// tenantCtx carries the tenant the way the application layer does.
func tenantCtx(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

// newAssessment builds an assessment directly rather than through the domain constructor.
func newAssessment(tenantID, academicYearID, subjectID string) *domain.Assessment {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.Assessment{
		ID:             uuid.NewString(),
		TenantID:       tenantID,
		AcademicYearID: academicYearID,
		SubjectID:      subjectID,
		Type:           string(domain.TypeAssignment),
		Title:          "Test Assessment",
		MaxScore:       100,
		Status:         string(domain.StatusDraft),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// newScore builds a score directly.
func newScore(tenantID, assessmentID, studentID string) *domain.Score {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.Score{
		ID:           uuid.NewString(),
		TenantID:     tenantID,
		AssessmentID: assessmentID,
		StudentID:    studentID,
		Score:        75,
		RecordedBy:   recordedByID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

var recordedByID = uuid.NewString()

func runAssessmentContract(t *testing.T, repo assessmentRepo) {
	t.Run("create assessment then read back within the tenant", func(t *testing.T) {
		a := newAssessment("upshs", uuid.NewString(), uuid.NewString())
		if err := repo.CreateAssessment(tenantCtx("upshs"), "upshs", a); err != nil {
			t.Fatalf("create: %v", err)
		}
		got, err := repo.GetAssessmentByID(tenantCtx("upshs"), "upshs", a.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Title != "Test Assessment" || got.TenantID != "upshs" {
			t.Fatalf("round trip changed the record: %+v", got)
		}
	})

	t.Run("another tenant cannot read the assessment", func(t *testing.T) {
		a := newAssessment("upshs", uuid.NewString(), uuid.NewString())
		if err := repo.CreateAssessment(tenantCtx("upshs"), "upshs", a); err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := repo.GetAssessmentByID(tenantCtx("aboom"), "aboom", a.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant read returned %v; want ErrNotFound", err)
		}
	})

	t.Run("another tenant cannot update or delete the assessment", func(t *testing.T) {
		a := newAssessment("upshs", uuid.NewString(), uuid.NewString())
		if err := repo.CreateAssessment(tenantCtx("upshs"), "upshs", a); err != nil {
			t.Fatalf("create: %v", err)
		}
		clone := *a
		clone.Title = "Hijacked"
		if err := repo.UpdateAssessment(tenantCtx("aboom"), "aboom", &clone); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant update returned %v; want ErrNotFound", err)
		}
		if err := repo.DeleteAssessment(tenantCtx("aboom"), "aboom", a.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant delete returned %v; want ErrNotFound", err)
		}
		got, err := repo.GetAssessmentByID(tenantCtx("upshs"), "upshs", a.ID)
		if err != nil || got.Title == "Hijacked" {
			t.Fatalf("the owner's record was altered across tenants: %+v (err=%v)", got, err)
		}
	})

	t.Run("delete removes the assessment from reads", func(t *testing.T) {
		a := newAssessment("upshs", uuid.NewString(), uuid.NewString())
		if err := repo.CreateAssessment(tenantCtx("upshs"), "upshs", a); err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := repo.DeleteAssessment(tenantCtx("upshs"), "upshs", a.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := repo.GetAssessmentByID(tenantCtx("upshs"), "upshs", a.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a deleted assessment is still readable: %v", err)
		}
	})

	t.Run("list assessments is tenant scoped and pages by cursor", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		ay := uuid.NewString()
		subj := uuid.NewString()
		for i := 0; i < 3; i++ {
			if err := repo.CreateAssessment(ctx, tenant, newAssessment(tenant, ay, subj)); err != nil {
				t.Fatalf("create: %v", err)
			}
		}
		seen := map[string]bool{}
		cursor := ""
		for range 20 {
			page, next, err := repo.ListAssessments(ctx, tenant, ports.AssessmentListFilter{
				Limit: 2, Cursor: cursor,
			})
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			for _, a := range page {
				if a.TenantID != tenant {
					t.Fatalf("list leaked a record owned by %q", a.TenantID)
				}
				if seen[a.ID] {
					t.Fatalf("the cursor repeated record %s", a.ID)
				}
				seen[a.ID] = true
			}
			if next == "" {
				break
			}
			cursor = next
		}
		if len(seen) < 3 {
			t.Fatalf("paging saw %d records; want at least 3", len(seen))
		}
	})

	t.Run("create score then read back within the tenant", func(t *testing.T) {
		a := newAssessment("upshs", uuid.NewString(), uuid.NewString())
		if err := repo.CreateAssessment(tenantCtx("upshs"), "upshs", a); err != nil {
			t.Fatalf("create assessment: %v", err)
		}
		s := newScore("upshs", a.ID, uuid.NewString())
		if err := repo.CreateScore(tenantCtx("upshs"), "upshs", s); err != nil {
			t.Fatalf("create score: %v", err)
		}
		got, err := repo.GetScoreByID(tenantCtx("upshs"), "upshs", a.ID, s.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Score != 75 || got.TenantID != "upshs" {
			t.Fatalf("round trip changed the record: %+v", got)
		}
	})

	t.Run("another tenant cannot read the score", func(t *testing.T) {
		a := newAssessment("upshs", uuid.NewString(), uuid.NewString())
		if err := repo.CreateAssessment(tenantCtx("upshs"), "upshs", a); err != nil {
			t.Fatalf("create assessment: %v", err)
		}
		s := newScore("upshs", a.ID, uuid.NewString())
		if err := repo.CreateScore(tenantCtx("upshs"), "upshs", s); err != nil {
			t.Fatalf("create score: %v", err)
		}
		if _, err := repo.GetScoreByID(tenantCtx("aboom"), "aboom", a.ID, s.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant read returned %v; want ErrNotFound", err)
		}
	})

	t.Run("a lifecycle commit queues exactly one claimable event", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		drain(t, repo)

		a := newAssessment(tenant, uuid.NewString(), uuid.NewString())
		payload := ports.AssessmentEventData(a, map[string]any{"mutation": "create"})
		m := ports.LifecycleMutation{
			Kind:       ports.AssessmentMutationCreate,
			Assessment: a,
		}
		if err := repo.CommitAssessmentLifecycle(ctx, tenant, m, []ports.LifecycleEvent{
			{EventType: "assessment.created.v1", Payload: payload},
		}); err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}

		if _, err := repo.GetAssessmentByID(ctx, tenant, a.ID); err != nil {
			t.Fatalf("lifecycle commit did not persist the record: %v", err)
		}

		claimed, err := repo.ClaimPendingAssessmentEvents(ctx, 100)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if len(claimed) != 1 {
			t.Fatalf("claimed %d events; want the one just queued", len(claimed))
		}
		if claimed[0].EventType != "assessment.created.v1" {
			t.Fatalf("unexpected event type %q", claimed[0].EventType)
		}

		if err := repo.MarkAssessmentEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("mark published: %v", err)
		}
		again, err := repo.ClaimPendingAssessmentEvents(ctx, 100)
		if err != nil {
			t.Fatalf("second claim: %v", err)
		}
		if len(again) != 0 {
			t.Fatalf("a published event was claimed again: %+v", again)
		}
	})

	t.Run("a failed event is retried rather than dropped", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		drain(t, repo)

		a := newAssessment(tenant, uuid.NewString(), uuid.NewString())
		payload := ports.AssessmentEventData(a, nil)
		m := ports.LifecycleMutation{
			Kind:       ports.AssessmentMutationCreate,
			Assessment: a,
		}
		if err := repo.CommitAssessmentLifecycle(ctx, tenant, m, []ports.LifecycleEvent{
			{EventType: "assessment.created.v1", Payload: payload},
		}); err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}
		claimed, err := repo.ClaimPendingAssessmentEvents(ctx, 100)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("claim: %d events, err=%v", len(claimed), err)
		}
		if err := repo.MarkAssessmentEventFailed(ctx, claimed[0].ID, "bus unavailable"); err != nil {
			t.Fatalf("mark failed: %v", err)
		}

		if err := repo.MarkAssessmentEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("a failed event could not be acknowledged later: %v", err)
		}
	})

	t.Run("score lifecycle with create and update mutations", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		drain(t, repo)

		a := newAssessment(tenant, uuid.NewString(), uuid.NewString())
		if err := repo.CreateAssessment(ctx, tenant, a); err != nil {
			t.Fatalf("create assessment: %v", err)
		}

		s := newScore(tenant, a.ID, uuid.NewString())
		payload := ports.ScoreEventData(s, map[string]any{"mutation": "create"})
		m := ports.LifecycleMutation{
			Kind:  ports.AssessmentMutationScoreCreate,
			Score: s,
		}
		if err := repo.CommitAssessmentLifecycle(ctx, tenant, m, []ports.LifecycleEvent{
			{EventType: "score.created.v1", Payload: payload},
		}); err != nil {
			t.Fatalf("commit score create: %v", err)
		}

		got, err := repo.GetScoreByID(ctx, tenant, a.ID, s.ID)
		if err != nil {
			t.Fatalf("get score: %v", err)
		}
		if got.Score != 75 {
			t.Fatalf("score not persisted: %+v", got)
		}

		got.Score = 85
		payloadUpd := ports.ScoreEventData(got, map[string]any{"mutation": "update"})
		mUpd := ports.LifecycleMutation{
			Kind:  ports.AssessmentMutationScoreUpdate,
			Score: got,
		}
		if err := repo.CommitAssessmentLifecycle(ctx, tenant, mUpd, []ports.LifecycleEvent{
			{EventType: "score.updated.v1", Payload: payloadUpd},
		}); err != nil {
			t.Fatalf("commit score update: %v", err)
		}

		updated, err := repo.GetScoreByID(ctx, tenant, a.ID, s.ID)
		if err != nil {
			t.Fatalf("get updated score: %v", err)
		}
		if updated.Score != 85 {
			t.Fatalf("score not updated: %+v", updated)
		}
	})
}

// drain empties the outbox so a subtest counts only the events it queued.
func drain(t *testing.T, repo assessmentRepo) {
	t.Helper()
	ctx := tenantCtx("upshs")
	for range 50 {
		claimed, err := repo.ClaimPendingAssessmentEvents(ctx, 100)
		if err != nil {
			t.Fatalf("drain: %v", err)
		}
		if len(claimed) == 0 {
			return
		}
		for _, e := range claimed {
			if err := repo.MarkAssessmentEventPublished(ctx, e.ID); err != nil {
				t.Fatalf("drain acknowledge: %v", err)
			}
		}
	}
}

func TestPostgresAssessmentRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	pg := testkit.NewPostgres(ctx, t, "../../migrations")
	runAssessmentContract(t, pgadapter.NewRepository(pg.DB))
}

func TestMongoAssessmentRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	mg := testkit.NewMongoReplicaSet(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, mg.Store); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	runAssessmentContract(t, mongoadapter.NewRepository(mg.Store))
}
