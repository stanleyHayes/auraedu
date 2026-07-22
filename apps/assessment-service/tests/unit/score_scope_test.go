package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/assessment-service/internal/application"
	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/assessment-service/internal/ports"
)

type scoreScope struct {
	ids      []string
	classIDs []string
}

func (s scoreScope) Resolve(context.Context, string, string, string) (ports.LearnerScope, error) {
	return ports.LearnerScope{StudentIDs: s.ids, ClassIDs: s.classIDs}, nil
}

func newScopedAssessment(t *testing.T, title string, maxScore int) *domain.Assessment {
	t.Helper()
	assessment, err := domain.NewAssessment(tenantA, ay1, subject1, "test", title, "", maxScore, nil)
	if err != nil {
		t.Fatal(err)
	}
	return assessment
}

func saveScopedAssessment(t *testing.T, repo ports.Repository, assessment *domain.Assessment) {
	t.Helper()
	if err := repo.CreateAssessment(ctxFor(tenantA), tenantA, assessment); err != nil {
		t.Fatal(err)
	}
}

func newScopedScore(t *testing.T, assessmentID, studentID string, score int) *domain.Score {
	t.Helper()
	value, err := domain.NewScore(tenantA, assessmentID, studentID, score, "teacher", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func saveScopedScore(t *testing.T, repo ports.Repository, score *domain.Score) {
	t.Helper()
	if err := repo.CreateScore(ctxFor(tenantA), tenantA, score); err != nil {
		t.Fatal(err)
	}
}

func TestTeacherAssessmentReadsAreRestrictedToAssignedClasses(t *testing.T) {
	repo := newFakeRepo()
	assigned := newScopedAssessment(t, "Assigned", 100)
	assigned.ClassIDs = []string{"class-own"}
	other := newScopedAssessment(t, "Other", 100)
	other.ClassIDs = []string{"class-other"}
	saveScopedAssessment(t, repo, assigned)
	saveScopedAssessment(t, repo, other)
	svc := application.NewService(repo, application.WithFeatureGate(enabledGates()), application.WithLearnerScopeResolver(scoreScope{classIDs: []string{"class-own"}}))
	teacher := actor(tenantA, application.PermRead)
	teacher.Role = "teacher"
	items, _, err := svc.ListAssessments(ctxFor(tenantA), teacher, ports.AssessmentListFilter{Limit: 20})
	if err != nil || len(items) != 1 || items[0].ID != assigned.ID {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if _, err := svc.GetAssessment(ctxFor(tenantA), teacher, other.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unassigned=%v", err)
	}
}

func TestTeacherScoreWritesAreRestrictedToAssignedRoster(t *testing.T) {
	repo := newFakeRepo()
	assessment := newScopedAssessment(t, "Midterm", 100)
	saveScopedAssessment(t, repo, assessment)
	svc := application.NewService(repo, application.WithFeatureGate(enabledGates()), application.WithLearnerScopeResolver(scoreScope{ids: []string{"student-own"}}))
	teacher := actor(tenantA, application.PermRecordScores)
	teacher.Role = "teacher"
	_, err := svc.CreateScore(ctxFor(tenantA), teacher, application.CreateScoreRequest{AssessmentID: assessment.ID, StudentID: "student-other", Score: 80, RecordedBy: teacher.UserID})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unassigned score write=%v", err)
	}
}

func TestCreateScorePublishesCompleteAnalyticsContext(t *testing.T) {
	repo := newFakeRepo()
	assessment := newScopedAssessment(t, "Midterm", 80)
	assessment.ClassIDs = []string{class1, class2}
	saveScopedAssessment(t, repo, assessment)
	pub := &fakePublisher{}
	svc := application.NewService(
		repo,
		application.WithFeatureGate(enabledGates()),
		application.WithPublisher(pub),
	)
	teacher := actor(tenantA, application.PermRecordScores)

	score, err := svc.CreateScore(ctxFor(tenantA), teacher, application.CreateScoreRequest{
		AssessmentID: assessment.ID,
		StudentID:    student1,
		Score:        72,
		RecordedBy:   teacher.UserID,
	})
	if err != nil {
		t.Fatalf("create score: %v", err)
	}
	if len(pub.scoreEv) != 1 {
		t.Fatalf("expected one score event, got %+v", pub.scoreEv)
	}
	event := pub.scoreEv[0]
	if event.eventType != "assessment.score_recorded.v1" || event.score.ID != score.ID {
		t.Fatalf("unexpected score event: %+v", event)
	}
	for key, want := range map[string]any{
		"assessment_id":    assessment.ID,
		"subject_id":       subject1,
		"academic_year_id": ay1,
		"max_score":        80,
	} {
		if got := event.meta[key]; got != want {
			t.Fatalf("meta[%s] = %v, want %v", key, got, want)
		}
	}
	if got, ok := event.meta["recorded_at"].(string); !ok || got == "" {
		t.Fatalf("recorded_at missing from score event: %+v", event.meta)
	}
	classIDs, ok := event.meta["class_ids"].([]string)
	if !ok || len(classIDs) != 2 || classIDs[0] != class1 || classIDs[1] != class2 {
		t.Fatalf("class_ids missing from score event: %+v", event.meta)
	}
}

func TestParentScoresAreRestrictedToLinkedStudents(t *testing.T) {
	repo := newFakeRepo()
	assessment := newScopedAssessment(t, "Midterm", 100)
	saveScopedAssessment(t, repo, assessment)
	own := newScopedScore(t, assessment.ID, "student-own", 80)
	other := newScopedScore(t, assessment.ID, "student-other", 90)
	saveScopedScore(t, repo, own)
	saveScopedScore(t, repo, other)
	svc := application.NewService(repo, application.WithFeatureGate(enabledGates()), application.WithLearnerScopeResolver(scoreScope{ids: []string{"student-own"}}))
	parent := actor(tenantA, application.PermRead)
	parent.Role = "parent"
	items, _, err := svc.ListScores(ctxFor(tenantA), parent, assessment.ID, ports.ScoreListFilter{Limit: 20})
	if err != nil || len(items) != 1 || items[0].StudentID != "student-own" {
		t.Fatalf("scores=%+v err=%v", items, err)
	}
	if _, err = svc.GetScore(ctxFor(tenantA), parent, assessment.ID, other.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unlinked score=%v", err)
	}
}

func TestStudentScoresFailClosedWithoutResolver(t *testing.T) {
	svc := application.NewService(newFakeRepo(), application.WithFeatureGate(enabledGates()))
	student := actor(tenantA, application.PermRead)
	student.Role = "student"
	if _, _, err := svc.ListScores(ctxFor(tenantA), student, "assessment", ports.ScoreListFilter{Limit: 20}); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("unconfigured=%v", err)
	}
}

func TestLearnerAssessmentReadsArePublishedAndClassScoped(t *testing.T) {
	repo := newFakeRepo()
	published := newScopedAssessment(t, "Published", 100)
	published.ClassIDs = []string{"class-own"}
	published.Status = string(domain.StatusPublished)
	draft := newScopedAssessment(t, "Draft", 100)
	draft.ClassIDs = []string{"class-own"}
	other := newScopedAssessment(t, "Other", 100)
	other.ClassIDs = []string{"class-other"}
	saveScopedAssessment(t, repo, published)
	saveScopedAssessment(t, repo, draft)
	saveScopedAssessment(t, repo, other)
	svc := application.NewService(repo, application.WithFeatureGate(enabledGates()), application.WithLearnerScopeResolver(scoreScope{classIDs: []string{"class-own"}}))
	student := actor(tenantA, application.PermRead)
	student.Role = "student"

	items, _, err := svc.ListAssessments(ctxFor(tenantA), student, ports.AssessmentListFilter{Limit: 20})
	if err != nil || len(items) != 1 || items[0].ID != published.ID {
		t.Fatalf("published scoped assessments=%+v err=%v", items, err)
	}
	if _, err = svc.GetAssessment(ctxFor(tenantA), student, draft.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("draft assessment=%v", err)
	}
	if _, err = svc.GetAssessment(ctxFor(tenantA), student, other.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other-class assessment=%v", err)
	}
}

func TestGradebookReadsEnforceLearnerAndTeacherScope(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo, application.WithFeatureGate(enabledGates()), application.WithLearnerScopeResolver(scoreScope{ids: []string{"student-own"}, classIDs: []string{"class-own"}}))
	student := actor(tenantA, application.PermRead)
	student.Role = "student"
	if _, err := svc.GetGradebook(ctxFor(tenantA), student, ports.GradebookFilter{StudentID: "student-own"}); err != nil {
		t.Fatalf("own gradebook=%v", err)
	}
	if _, err := svc.GetGradebook(ctxFor(tenantA), student, ports.GradebookFilter{StudentID: "student-other"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other gradebook=%v", err)
	}
	if _, err := svc.GetGradebook(ctxFor(tenantA), student, ports.GradebookFilter{ClassID: "class-own"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("student class gradebook=%v", err)
	}

	teacher := actor(tenantA, application.PermRead)
	teacher.Role = "teacher"
	if _, err := svc.GetGradebook(ctxFor(tenantA), teacher, ports.GradebookFilter{ClassID: "class-own"}); err != nil {
		t.Fatalf("assigned class gradebook=%v", err)
	}
	if _, err := svc.GetGradebook(ctxFor(tenantA), teacher, ports.GradebookFilter{ClassID: "class-other"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unassigned class gradebook=%v", err)
	}
}
