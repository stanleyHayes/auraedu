package integration

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/assessment-service/internal/application"
	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/assessment-service/internal/ports"
	"github.com/auraedu/platform/flags"
)

const (
	class1 = "11111111-aaaa-4aaa-8aaa-aaaaaaaaaaa1"
	class2 = "22222222-bbbb-4bbb-8bbb-bbbbbbbbbbb2"
)

func mustCreateAssignment(ctx context.Context, t *testing.T, repo ports.Repository, academicYearID, subjectID, title string, maxScore int, classIDs []string) *domain.Assessment {
	t.Helper()
	a, err := domain.NewAssignment(tenantA, academicYearID, subjectID, title, "", maxScore, nil, classIDs)
	if err != nil {
		t.Fatalf("new assignment: %v", err)
	}
	if err := repo.CreateAssessment(ctx, tenantA, a); err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	return a
}

func assignmentSvc(repo ports.Repository, tenantID string) *application.Service {
	gates := flags.NewStaticSnapshot()
	gates.Set(tenantID, application.FeatureAssessments, true)
	gates.Set(tenantID, application.FeatureAssignments, true)
	return application.NewService(repo, application.WithFeatureGate(gates))
}

func TestRepository_AssignmentClassIDsRoundTrip(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	due := time.Date(2025, 11, 1, 23, 59, 0, 0, time.UTC)
	a, err := domain.NewAssignment(tenantA, ay1, subject1, "Essay 1", "Write 500 words", 50, &due, []string{class1, class2})
	if err != nil {
		t.Fatalf("new assignment: %v", err)
	}
	if err := repo.CreateAssessment(ctx, tenantA, a); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetAssessmentByID(ctx, tenantA, a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.ClassIDs) != 2 || got.ClassIDs[0] != class1 || got.ClassIDs[1] != class2 {
		t.Fatalf("class_ids not round-tripped: %v", got.ClassIDs)
	}
	if got.PublishedAt != nil {
		t.Fatalf("expected nil published_at, got %v", got.PublishedAt)
	}
	if got.DueDate == nil || !got.DueDate.Equal(due) {
		t.Fatalf("due_date not round-tripped: %v", got.DueDate)
	}
}

func TestRepository_ListAssignmentsFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	a1 := mustCreateAssignment(ctx, t, repo, ay1, subject1, "Essay 1", 50, []string{class1})
	mustCreateAssignment(ctx, t, repo, ay1, subject2, "Essay 2", 50, []string{class2})
	mustCreateAssessment(ctx, t, repo, ay1, subject1, "test", "Midterm", 100)
	mustCreateScore(ctx, t, repo, a1.ID, studentA, 40)

	cases := []struct {
		name   string
		filter ports.AssignmentListFilter
		want   int
	}{
		{"all assignments only", ports.AssignmentListFilter{Limit: 10}, 2},
		{"by subject_id", ports.AssignmentListFilter{Limit: 10, SubjectID: subject1}, 1},
		{"by class_id", ports.AssignmentListFilter{Limit: 10, ClassID: class2}, 1},
		{"by student_id", ports.AssignmentListFilter{Limit: 10, StudentID: studentA}, 1},
		{"by student_id without scores", ports.AssignmentListFilter{Limit: 10, StudentID: studentB}, 0},
		{"by status", ports.AssignmentListFilter{Limit: 10, Status: "draft"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, _, err := repo.ListAssignments(ctx, tenantA, tc.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(page) != tc.want {
				t.Fatalf("expected %d assignments, got %d", tc.want, len(page))
			}
			for _, a := range page {
				if !a.IsAssignment() {
					t.Fatalf("non-assignment leaked into assignments list: %+v", a)
				}
			}
		})
	}
}

func TestService_AssignmentPublishFlow(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)
	svc := assignmentSvc(repo, tenantA)
	manager := actorWithPerms(tenantA, application.PermManage)

	a, err := svc.CreateAssignment(ctx, manager, application.CreateAssignmentRequest{
		AcademicYearID: ay1,
		SubjectID:      subject1,
		Title:          "Essay 1",
		Instructions:   "Write 500 words",
		MaxScore:       50,
		ClassIDs:       []string{class1},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.PublishedAt != nil || a.Status != string(domain.StatusDraft) {
		t.Fatalf("expected draft, got %+v", a)
	}

	published, err := svc.PublishAssignment(ctx, manager, a.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Status != string(domain.StatusPublished) || published.PublishedAt == nil {
		t.Fatalf("expected published, got %+v", published)
	}

	// published_at survives a fresh read from Postgres.
	got, err := repo.GetAssessmentByID(ctx, tenantA, a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PublishedAt == nil || got.Status != string(domain.StatusPublished) {
		t.Fatalf("published state not persisted: %+v", got)
	}

	if _, err := svc.PublishAssignment(ctx, manager, a.ID); err == nil {
		t.Fatal("expected error publishing twice")
	}
}

func TestService_AssignmentsFlagGatesAgainstRealDB(t *testing.T) {
	ctx := withTenant(context.Background(), tenantB)
	repo := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantB, application.FeatureAssessments, true)
	gates.Set(tenantB, application.FeatureAssignments, false)
	svc := application.NewService(repo, application.WithFeatureGate(gates))

	_, err := svc.CreateAssignment(ctx, actorWithPerms(tenantB, application.PermManage), application.CreateAssignmentRequest{
		AcademicYearID: ay1,
		SubjectID:      subject1,
		Title:          "Essay",
		MaxScore:       50,
	})
	if err == nil {
		t.Fatal("expected feature-disabled error when assignments flag is off")
	}
}

func TestRepository_TenantIsolation_Assignments(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	a := mustCreateAssignment(aCtx, t, repo, ay1, subject1, "Essay 1", 50, []string{class1})

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetAssessmentByID(bCtx, tenantB, a.ID); err == nil {
		t.Fatal("tenant B should not see tenant A assignment")
	}
	list, _, err := repo.ListAssignments(bCtx, tenantB, ports.AssignmentListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 assignments, got %d", len(list))
	}
}

func TestService_Gradebook(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)
	svc := assignmentSvc(repo, tenantA)
	manager := actorWithPerms(tenantA, application.PermManage)
	scorer := actorWithPerms(tenantA, application.PermRecordScores)
	reader := actorWithPerms(tenantA, application.PermRead)

	// subject1: a test (max 100) and an assignment tagged class1 (max 50).
	test, err := svc.CreateAssessment(ctx, manager, application.CreateAssessmentRequest{
		AcademicYearID: ay1, SubjectID: subject1, Type: "test", Title: "Midterm", MaxScore: 100,
	})
	if err != nil {
		t.Fatalf("create test: %v", err)
	}
	assignment, err := svc.CreateAssignment(ctx, manager, application.CreateAssignmentRequest{
		AcademicYearID: ay1, SubjectID: subject1, Title: "Essay", MaxScore: 50, ClassIDs: []string{class1},
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	// subject2 in another academic year.
	other, err := svc.CreateAssessment(ctx, manager, application.CreateAssessmentRequest{
		AcademicYearID: ay2, SubjectID: subject2, Type: "exam", Title: "Final", MaxScore: 100,
	})
	if err != nil {
		t.Fatalf("create exam: %v", err)
	}

	mustScore := func(assessmentID string, score int) {
		t.Helper()
		if _, err := svc.CreateScore(ctx, scorer, application.CreateScoreRequest{
			AssessmentID: assessmentID, StudentID: studentA, Score: score, RecordedBy: staff1,
		}); err != nil {
			t.Fatalf("record score: %v", err)
		}
	}
	mustScore(test.ID, 80)       // 80%
	mustScore(assignment.ID, 40) // 80%
	mustScore(other.ID, 60)      // 60%

	// Per-student, unfiltered: two subjects, overall weighted (80+40+60)/(100+50+100)=72.
	book, err := svc.GetGradebook(ctx, reader, ports.GradebookFilter{StudentID: studentA})
	if err != nil {
		t.Fatalf("gradebook: %v", err)
	}
	if len(book.Subjects) != 2 {
		t.Fatalf("expected 2 subjects, got %+v", book.Subjects)
	}
	if book.Overall.AssessmentCount != 3 || book.Overall.TotalScore != 180 || book.Overall.TotalMaxScore != 250 {
		t.Fatalf("wrong overall totals: %+v", book.Overall)
	}
	if book.Overall.WeightedAverage == nil || *book.Overall.WeightedAverage != 72 {
		t.Fatalf("expected weighted average 72, got %v", book.Overall.WeightedAverage)
	}
	// Simple average: (80 + 80 + 60) / 3 = 73.33.
	if book.Overall.Average == nil || *book.Overall.Average != 73.33 {
		t.Fatalf("expected average 73.33, got %v", book.Overall.Average)
	}

	// Academic-year filter restricts to ay1 (subject1 only).
	book, err = svc.GetGradebook(ctx, reader, ports.GradebookFilter{StudentID: studentA, AcademicYearID: ay1})
	if err != nil {
		t.Fatalf("gradebook ay1: %v", err)
	}
	if len(book.Subjects) != 1 || book.Subjects[0].SubjectID != subject1 {
		t.Fatalf("expected only subject1 for ay1, got %+v", book.Subjects)
	}
	if book.Overall.AssessmentCount != 2 {
		t.Fatalf("expected 2 assessments in ay1, got %+v", book.Overall)
	}

	// Per-class: only the assignment tagged class1 counts.
	book, err = svc.GetGradebook(ctx, reader, ports.GradebookFilter{ClassID: class1})
	if err != nil {
		t.Fatalf("gradebook class1: %v", err)
	}
	if book.Overall.AssessmentCount != 1 || book.Overall.TotalScore != 40 || book.Overall.TotalMaxScore != 50 {
		t.Fatalf("wrong class gradebook: %+v", book.Overall)
	}
	if book.Overall.WeightedAverage == nil || *book.Overall.WeightedAverage != 80 {
		t.Fatalf("expected class weighted average 80, got %v", book.Overall.WeightedAverage)
	}

	// Student with no scores: empty gradebook with nil averages.
	book, err = svc.GetGradebook(ctx, reader, ports.GradebookFilter{StudentID: studentB})
	if err != nil {
		t.Fatalf("gradebook empty: %v", err)
	}
	if len(book.Subjects) != 0 || book.Overall.Average != nil || book.Overall.WeightedAverage != nil {
		t.Fatalf("expected empty gradebook, got %+v", book)
	}
}

func TestRepository_TenantIsolation_Gradebook(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	a := mustCreateAssessment(aCtx, t, repo, ay1, subject1, "test", "Midterm", 100)
	mustCreateScore(aCtx, t, repo, a.ID, studentA, 85)

	bCtx := withTenant(ctx, tenantB)
	rows, err := repo.GradebookScores(bCtx, tenantB, ports.GradebookFilter{StudentID: studentA})
	if err != nil {
		t.Fatalf("gradebook tenant B: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("tenant B should see 0 gradebook rows, got %d", len(rows))
	}
}
