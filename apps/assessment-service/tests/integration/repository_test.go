package integration

import (
	"context"
	"testing"

	"github.com/auraedu/assessment-service/internal/adapters/postgres"
	"github.com/auraedu/assessment-service/internal/application"
	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/assessment-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const tenantA = "11111111-1111-1111-1111-111111111111"
const tenantB = "22222222-2222-2222-2222-222222222222"

const ay1 = "cccccccc-cccc-cccc-cccc-cccccccccccc"
const ay2 = "dddddddd-dddd-dddd-dddd-dddddddddddd"
const subject1 = "55555555-5555-5555-5555-555555555555"
const subject2 = "66666666-6666-6666-6666-666666666666"
const studentA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
const studentB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
const staff1 = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
const staff2 = "ffffffff-ffff-ffff-ffff-ffffffffffff"

func newRepo(t *testing.T) (ports.Repository, *testkit.PostgresTestDB) {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewRepository(tdb.DB), tdb
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func actorWithPerms(tenantID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantID, Permissions: perms}
}

func mustCreateAssessment(t *testing.T, ctx context.Context, repo ports.Repository, tenantID, academicYearID, subjectID, assessmentType, title string, maxScore int) *domain.Assessment {
	t.Helper()
	a, err := domain.NewAssessment(tenantID, academicYearID, subjectID, assessmentType, title, "", maxScore, nil)
	if err != nil {
		t.Fatalf("new assessment: %v", err)
	}
	if err := repo.CreateAssessment(ctx, tenantID, a); err != nil {
		t.Fatalf("create assessment: %v", err)
	}
	return a
}

func mustCreateScore(t *testing.T, ctx context.Context, repo ports.Repository, tenantID, assessmentID, studentID string, score int) *domain.Score {
	t.Helper()
	s, err := domain.NewScore(tenantID, assessmentID, studentID, score, staff1, "", 100)
	if err != nil {
		t.Fatalf("new score: %v", err)
	}
	if err := repo.CreateScore(ctx, tenantID, s); err != nil {
		t.Fatalf("create score: %v", err)
	}
	return s
}

func TestRepository_CreateAndGetAssessment(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	a := mustCreateAssessment(t, ctx, repo, tenantA, ay1, subject1, "test", "Midterm", 100)

	got, err := repo.GetAssessmentByID(ctx, tenantA, a.ID)
	if err != nil {
		t.Fatalf("get assessment: %v", err)
	}
	if got.ID != a.ID || got.Title != "Midterm" {
		t.Fatalf("assessment mismatch: %+v", got)
	}
}

func TestRepository_ListAssessmentPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	mustCreateAssessment(t, ctx, repo, tenantA, ay1, subject1, "test", "Quiz 1", 20)
	a2 := mustCreateAssessment(t, ctx, repo, tenantA, ay1, subject1, "test", "Quiz 2", 20)

	page, next, err := repo.ListAssessments(ctx, tenantA, ports.AssessmentListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	page2, _, err := repo.ListAssessments(ctx, tenantA, ports.AssessmentListFilter{Limit: 1, Cursor: next})
	if err != nil {
		t.Fatalf("list cursor: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != a2.ID {
		t.Fatalf("expected second assessment, got %+v", page2)
	}
}

func TestRepository_ListAssessmentFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	mustCreateAssessment(t, ctx, repo, tenantA, ay1, subject1, "test", "Math Test", 100)
	mustCreateAssessment(t, ctx, repo, tenantA, ay1, subject2, "assignment", "English Essay", 50)
	mustCreateAssessment(t, ctx, repo, tenantA, ay2, subject1, "exam", "Science Exam", 100)

	cases := []struct {
		name   string
		filter ports.AssessmentListFilter
		want   int
	}{
		{"by academic_year_id", ports.AssessmentListFilter{Limit: 10, AcademicYearID: ay1}, 2},
		{"by subject_id", ports.AssessmentListFilter{Limit: 10, SubjectID: subject1}, 2},
		{"by type", ports.AssessmentListFilter{Limit: 10, Type: "test"}, 1},
		{"by status", ports.AssessmentListFilter{Limit: 10, Status: "draft"}, 3},
		{"combined", ports.AssessmentListFilter{Limit: 10, AcademicYearID: ay1, SubjectID: subject1}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, _, err := repo.ListAssessments(ctx, tenantA, tc.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(page) != tc.want {
				t.Fatalf("expected %d assessments, got %d", tc.want, len(page))
			}
		})
	}
}

func TestRepository_UpdateAssessment(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	a := mustCreateAssessment(t, ctx, repo, tenantA, ay1, subject1, "test", "Midterm", 100)
	title := "Final Exam"
	status := string(domain.StatusPublished)
	if _, err := a.ApplyUpdate(&title, nil, nil, nil, nil, &status); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.UpdateAssessment(ctx, tenantA, a); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetAssessmentByID(ctx, tenantA, a.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Title != title || got.Status != status {
		t.Fatalf("assessment not updated: %+v", got)
	}
}

func TestRepository_DeleteAssessment(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	a := mustCreateAssessment(t, ctx, repo, tenantA, ay1, subject1, "test", "Midterm", 100)
	if err := repo.DeleteAssessment(ctx, tenantA, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetAssessmentByID(ctx, tenantA, a.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_CreateAndGetScore(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	a := mustCreateAssessment(t, ctx, repo, tenantA, ay1, subject1, "test", "Midterm", 100)
	s := mustCreateScore(t, ctx, repo, tenantA, a.ID, studentA, 85)

	got, err := repo.GetScoreByID(ctx, tenantA, a.ID, s.ID)
	if err != nil {
		t.Fatalf("get score: %v", err)
	}
	if got.ID != s.ID || got.Score != 85 {
		t.Fatalf("score mismatch: %+v", got)
	}
}

func TestRepository_ListScoresFilter(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	a := mustCreateAssessment(t, ctx, repo, tenantA, ay1, subject1, "test", "Midterm", 100)
	mustCreateScore(t, ctx, repo, tenantA, a.ID, studentA, 85)
	mustCreateScore(t, ctx, repo, tenantA, a.ID, studentB, 90)

	page, _, err := repo.ListScores(ctx, tenantA, a.ID, ports.ScoreListFilter{Limit: 10, StudentID: studentA})
	if err != nil {
		t.Fatalf("list scores: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 score, got %d", len(page))
	}
}

func TestRepository_UpdateScore(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	a := mustCreateAssessment(t, ctx, repo, tenantA, ay1, subject1, "test", "Midterm", 100)
	s := mustCreateScore(t, ctx, repo, tenantA, a.ID, studentA, 75)
	score := 88
	notes := "Remarked"
	if _, err := s.ApplyUpdate(&score, &notes, 100); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.UpdateScore(ctx, tenantA, s); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetScoreByID(ctx, tenantA, a.ID, s.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Score != score || got.Notes == nil || *got.Notes != notes {
		t.Fatalf("score not updated: %+v", got)
	}
}

func TestRepository_DeleteScore(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	a := mustCreateAssessment(t, ctx, repo, tenantA, ay1, subject1, "test", "Midterm", 100)
	s := mustCreateScore(t, ctx, repo, tenantA, a.ID, studentA, 85)
	if err := repo.DeleteScore(ctx, tenantA, a.ID, s.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetScoreByID(ctx, tenantA, a.ID, s.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_TenantIsolation_Assessments(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	a := mustCreateAssessment(t, aCtx, repo, tenantA, ay1, subject1, "test", "Midterm", 100)

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetAssessmentByID(bCtx, tenantB, a.ID); err == nil {
		t.Fatal("tenant B should not see tenant A assessment")
	}

	list, _, err := repo.ListAssessments(bCtx, tenantB, ports.AssessmentListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 assessments, got %d", len(list))
	}
}

func TestRepository_TenantIsolation_Scores(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	a := mustCreateAssessment(t, aCtx, repo, tenantA, ay1, subject1, "test", "Midterm", 100)
	s := mustCreateScore(t, aCtx, repo, tenantA, a.ID, studentA, 85)

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetScoreByID(bCtx, tenantB, a.ID, s.ID); err == nil {
		t.Fatal("tenant B should not see tenant A score")
	}

	list, _, err := repo.ListScores(bCtx, tenantB, a.ID, ports.ScoreListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant B scores: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 scores, got %d", len(list))
	}
}

func TestService_FeatureFlagGatesAccess(t *testing.T) {
	ctx := withTenant(context.Background(), tenantB)
	repo, _ := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantB, application.FeatureAssessments, false)

	svc := application.NewService(repo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantB, application.PermManage)

	_, err := svc.CreateAssessment(ctx, actor, application.CreateAssessmentRequest{
		AcademicYearID: ay1,
		SubjectID:      subject1,
		Type:           "test",
		Title:          "Midterm",
		MaxScore:       100,
	})
	if err == nil {
		t.Fatal("expected feature-disabled error")
	}
}

func TestService_FeatureFlagAllowsAccessWhenEnabled(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureAssessments, true)

	svc := application.NewService(repo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantA, application.PermManage)

	a, err := svc.CreateAssessment(ctx, actor, application.CreateAssessmentRequest{
		AcademicYearID: ay1,
		SubjectID:      subject1,
		Type:           "test",
		Title:          "Midterm",
		MaxScore:       100,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.ID == "" {
		t.Fatal("expected assessment id")
	}
}

func TestService_CreateScoreRejectsExcessScore(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureAssessments, true)

	svc := application.NewService(repo, application.WithFeatureGate(gates))
	manager := actorWithPerms(tenantA, application.PermManage)
	scorer := actorWithPerms(tenantA, application.PermRecordScores)

	a, err := svc.CreateAssessment(ctx, manager, application.CreateAssessmentRequest{
		AcademicYearID: ay1,
		SubjectID:      subject1,
		Type:           "test",
		Title:          "Midterm",
		MaxScore:       100,
	})
	if err != nil {
		t.Fatalf("create assessment: %v", err)
	}

	_, err = svc.CreateScore(ctx, scorer, application.CreateScoreRequest{
		AssessmentID: a.ID,
		StudentID:    studentA,
		Score:        101,
		RecordedBy:   staff1,
	})
	if err == nil {
		t.Fatal("expected score-exceeds-max_score error")
	}
}

func TestService_CreateScoreSucceedsWithinMaxScore(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureAssessments, true)

	svc := application.NewService(repo, application.WithFeatureGate(gates))
	manager := actorWithPerms(tenantA, application.PermManage)
	scorer := actorWithPerms(tenantA, application.PermRecordScores)

	a, err := svc.CreateAssessment(ctx, manager, application.CreateAssessmentRequest{
		AcademicYearID: ay1,
		SubjectID:      subject1,
		Type:           "test",
		Title:          "Midterm",
		MaxScore:       100,
	})
	if err != nil {
		t.Fatalf("create assessment: %v", err)
	}

	s, err := svc.CreateScore(ctx, scorer, application.CreateScoreRequest{
		AssessmentID: a.ID,
		StudentID:    studentA,
		Score:        95,
		RecordedBy:   staff1,
	})
	if err != nil {
		t.Fatalf("create score: %v", err)
	}
	if s.Score != 95 {
		t.Fatalf("score mismatch: got %d", s.Score)
	}
}
