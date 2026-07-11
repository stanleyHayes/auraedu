package integration

import (
	"context"
	"testing"

	"github.com/auraedu/cbt-service/internal/adapters/postgres"
	"github.com/auraedu/cbt-service/internal/application"
	"github.com/auraedu/cbt-service/internal/domain"
	"github.com/auraedu/cbt-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	tenantA  = "11111111-1111-1111-1111-111111111111"
	tenantB  = "22222222-2222-2222-2222-222222222222"
	ay1      = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	ay2      = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	subject1 = "55555555-5555-5555-5555-555555555555"
	subject2 = "66666666-6666-6666-6666-666666666666"
	studentA = "77777777-7777-7777-7777-777777777777"
	studentB = "88888888-8888-8888-8888-888888888888"
)

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

func actorStudent(tenantID, studentID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: studentID, TenantID: tenantID, Permissions: perms}
}

func mustCreateQuestion(t *testing.T, ctx context.Context, repo ports.Repository, tenantID, academicYearID, subjectID, qType, text, correct string, marks int, options []string) *domain.QuestionBank {
	t.Helper()
	q, err := domain.NewQuestionBank(tenantID, academicYearID, subjectID, text, qType, correct, marks, options)
	if err != nil {
		t.Fatalf("new question: %v", err)
	}
	if err := repo.CreateQuestion(ctx, tenantID, q); err != nil {
		t.Fatalf("create question: %v", err)
	}
	return q
}

func mustCreateExamSession(t *testing.T, ctx context.Context, repo ports.Repository, tenantID, title, academicYearID, subjectID string, questionIDs []string, duration int) *domain.ExamSession {
	t.Helper()
	e, err := domain.NewExamSession(tenantID, title, academicYearID, subjectID, questionIDs, duration, nil, nil)
	if err != nil {
		t.Fatalf("new exam session: %v", err)
	}
	if err := repo.CreateExamSession(ctx, tenantID, e); err != nil {
		t.Fatalf("create exam session: %v", err)
	}
	return e
}

func mustCreateSubmission(t *testing.T, ctx context.Context, repo ports.Repository, tenantID, examSessionID, studentID string) *domain.Submission {
	t.Helper()
	s, err := domain.NewSubmission(tenantID, examSessionID, studentID)
	if err != nil {
		t.Fatalf("new submission: %v", err)
	}
	if err := repo.CreateSubmission(ctx, tenantID, s); err != nil {
		t.Fatalf("create submission: %v", err)
	}
	return s
}

// --- Question bank integration. ---

func TestRepository_CreateAndGetQuestion(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	q := mustCreateQuestion(t, ctx, repo, tenantA, ay1, subject1, "multiple_choice", "What is 2+2?", "4", 2, []string{"3", "4", "5"})

	got, err := repo.GetQuestionByID(ctx, tenantA, q.ID)
	if err != nil {
		t.Fatalf("get question: %v", err)
	}
	if got.ID != q.ID || got.CorrectAnswer != "4" {
		t.Fatalf("question mismatch: %+v", got)
	}
}

func TestRepository_ListQuestionFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	mustCreateQuestion(t, ctx, repo, tenantA, ay1, subject1, "multiple_choice", "Q1", "a", 1, []string{"a", "b"})
	mustCreateQuestion(t, ctx, repo, tenantA, ay1, subject2, "multiple_choice", "Q2", "a", 1, []string{"a", "b"})
	mustCreateQuestion(t, ctx, repo, tenantA, ay2, subject1, "multiple_choice", "Q3", "a", 1, []string{"a", "b"})

	cases := []struct {
		name   string
		filter ports.QuestionListFilter
		want   int
	}{
		{"by academic_year_id", ports.QuestionListFilter{Limit: 10, AcademicYearID: ay1}, 2},
		{"by subject_id", ports.QuestionListFilter{Limit: 10, SubjectID: subject1}, 2},
		{"by status", ports.QuestionListFilter{Limit: 10, Status: "draft"}, 3},
		{"combined", ports.QuestionListFilter{Limit: 10, AcademicYearID: ay1, SubjectID: subject1}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, _, err := repo.ListQuestions(ctx, tenantA, tc.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(page) != tc.want {
				t.Fatalf("expected %d questions, got %d", tc.want, len(page))
			}
		})
	}
}

func TestRepository_UpdateQuestion(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	q := mustCreateQuestion(t, ctx, repo, tenantA, ay1, subject1, "multiple_choice", "Q1", "a", 1, []string{"a", "b"})
	text := "Updated question"
	marks := 5
	if _, err := q.ApplyUpdate(&text, nil, nil, &marks, nil, nil); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.UpdateQuestion(ctx, tenantA, q); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetQuestionByID(ctx, tenantA, q.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.QuestionText != text || got.Marks != marks {
		t.Fatalf("question not updated: %+v", got)
	}
}

func TestRepository_DeleteQuestion(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	q := mustCreateQuestion(t, ctx, repo, tenantA, ay1, subject1, "multiple_choice", "Q1", "a", 1, []string{"a", "b"})
	if err := repo.DeleteQuestion(ctx, tenantA, q.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetQuestionByID(ctx, tenantA, q.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

// --- Exam session integration. ---

func TestRepository_CreateAndGetExamSession(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	q := mustCreateQuestion(t, ctx, repo, tenantA, ay1, subject1, "multiple_choice", "Q1", "a", 1, []string{"a", "b"})
	e := mustCreateExamSession(t, ctx, repo, tenantA, "Midterm", ay1, subject1, []string{q.ID}, 60)

	got, err := repo.GetExamSessionByID(ctx, tenantA, e.ID)
	if err != nil {
		t.Fatalf("get exam session: %v", err)
	}
	if got.ID != e.ID || got.Title != "Midterm" {
		t.Fatalf("exam session mismatch: %+v", got)
	}
}

func TestRepository_ListExamSessionFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	q := mustCreateQuestion(t, ctx, repo, tenantA, ay1, subject1, "multiple_choice", "Q1", "a", 1, []string{"a", "b"})
	mustCreateExamSession(t, ctx, repo, tenantA, "E1", ay1, subject1, []string{q.ID}, 60)
	mustCreateExamSession(t, ctx, repo, tenantA, "E2", ay1, subject2, []string{q.ID}, 60)
	mustCreateExamSession(t, ctx, repo, tenantA, "E3", ay2, subject1, []string{q.ID}, 60)

	cases := []struct {
		name   string
		filter ports.ExamSessionListFilter
		want   int
	}{
		{"by academic_year_id", ports.ExamSessionListFilter{Limit: 10, AcademicYearID: ay1}, 2},
		{"by subject_id", ports.ExamSessionListFilter{Limit: 10, SubjectID: subject1}, 2},
		{"by status", ports.ExamSessionListFilter{Limit: 10, Status: "draft"}, 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, _, err := repo.ListExamSessions(ctx, tenantA, tc.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(page) != tc.want {
				t.Fatalf("expected %d exams, got %d", tc.want, len(page))
			}
		})
	}
}

func TestRepository_UpdateExamSession(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	q := mustCreateQuestion(t, ctx, repo, tenantA, ay1, subject1, "multiple_choice", "Q1", "a", 1, []string{"a", "b"})
	e := mustCreateExamSession(t, ctx, repo, tenantA, "Midterm", ay1, subject1, []string{q.ID}, 60)
	title := "Final"
	duration := 90
	status := string(domain.ExamStatusPublished)
	if _, err := e.ApplyUpdate(&title, nil, &duration, nil, nil, &status); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.UpdateExamSession(ctx, tenantA, e); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetExamSessionByID(ctx, tenantA, e.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Title != title || got.DurationMinutes != duration || got.Status != status {
		t.Fatalf("exam session not updated: %+v", got)
	}
}

func TestRepository_DeleteExamSession(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	q := mustCreateQuestion(t, ctx, repo, tenantA, ay1, subject1, "multiple_choice", "Q1", "a", 1, []string{"a", "b"})
	e := mustCreateExamSession(t, ctx, repo, tenantA, "Midterm", ay1, subject1, []string{q.ID}, 60)
	if err := repo.DeleteExamSession(ctx, tenantA, e.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetExamSessionByID(ctx, tenantA, e.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

// --- Submission integration. ---

func TestRepository_CreateAndGetSubmission(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	q := mustCreateQuestion(t, ctx, repo, tenantA, ay1, subject1, "multiple_choice", "Q1", "a", 1, []string{"a", "b"})
	e := mustCreateExamSession(t, ctx, repo, tenantA, "Midterm", ay1, subject1, []string{q.ID}, 60)
	s := mustCreateSubmission(t, ctx, repo, tenantA, e.ID, studentA)

	got, err := repo.GetSubmissionByID(ctx, tenantA, s.ID)
	if err != nil {
		t.Fatalf("get submission: %v", err)
	}
	if got.ID != s.ID || got.StudentID != studentA {
		t.Fatalf("submission mismatch: %+v", got)
	}
}

func TestRepository_GetSubmissionByExamAndStudent(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	q := mustCreateQuestion(t, ctx, repo, tenantA, ay1, subject1, "multiple_choice", "Q1", "a", 1, []string{"a", "b"})
	e := mustCreateExamSession(t, ctx, repo, tenantA, "Midterm", ay1, subject1, []string{q.ID}, 60)
	s := mustCreateSubmission(t, ctx, repo, tenantA, e.ID, studentA)

	got, err := repo.GetSubmissionByExamAndStudent(ctx, tenantA, e.ID, studentA)
	if err != nil {
		t.Fatalf("get by exam/student: %v", err)
	}
	if got.ID != s.ID {
		t.Fatalf("submission mismatch: %+v", got)
	}
}

func TestRepository_ListSubmissionsFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	q := mustCreateQuestion(t, ctx, repo, tenantA, ay1, subject1, "multiple_choice", "Q1", "a", 1, []string{"a", "b"})
	e := mustCreateExamSession(t, ctx, repo, tenantA, "Midterm", ay1, subject1, []string{q.ID}, 60)
	mustCreateSubmission(t, ctx, repo, tenantA, e.ID, studentA)
	mustCreateSubmission(t, ctx, repo, tenantA, e.ID, studentB)

	page, _, err := repo.ListSubmissions(ctx, tenantA, ports.SubmissionListFilter{Limit: 10, StudentID: studentA})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 submission, got %d", len(page))
	}
}

// --- Tenant isolation. ---

func TestRepository_TenantIsolation_Questions(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	q := mustCreateQuestion(t, aCtx, repo, tenantA, ay1, subject1, "multiple_choice", "Q1", "a", 1, []string{"a", "b"})

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetQuestionByID(bCtx, tenantB, q.ID); err == nil {
		t.Fatal("tenant B should not see tenant A question")
	}
	list, _, err := repo.ListQuestions(bCtx, tenantB, ports.QuestionListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 questions, got %d", len(list))
	}
}

func TestRepository_TenantIsolation_ExamSessions(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	q := mustCreateQuestion(t, aCtx, repo, tenantA, ay1, subject1, "multiple_choice", "Q1", "a", 1, []string{"a", "b"})
	e := mustCreateExamSession(t, aCtx, repo, tenantA, "Midterm", ay1, subject1, []string{q.ID}, 60)

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetExamSessionByID(bCtx, tenantB, e.ID); err == nil {
		t.Fatal("tenant B should not see tenant A exam session")
	}
}

func TestRepository_TenantIsolation_Submissions(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	q := mustCreateQuestion(t, aCtx, repo, tenantA, ay1, subject1, "multiple_choice", "Q1", "a", 1, []string{"a", "b"})
	e := mustCreateExamSession(t, aCtx, repo, tenantA, "Midterm", ay1, subject1, []string{q.ID}, 60)
	s := mustCreateSubmission(t, aCtx, repo, tenantA, e.ID, studentA)

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetSubmissionByID(bCtx, tenantB, s.ID); err == nil {
		t.Fatal("tenant B should not see tenant A submission")
	}
}

// --- Feature flag gating. ---

func TestService_FeatureFlagGatesAccess(t *testing.T) {
	ctx := withTenant(context.Background(), tenantB)
	repo, _ := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantB, application.FeatureCBTExams, false)

	svc := application.NewService(repo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantB, application.PermAuthor)

	_, err := svc.CreateQuestion(ctx, actor, application.CreateQuestionRequest{
		AcademicYearID: ay1,
		SubjectID:      subject1,
		QuestionText:   "Q?",
		QuestionType:   "multiple_choice",
		CorrectAnswer:  "a",
		Marks:          1,
		Options:        []string{"a", "b"},
	})
	if err == nil {
		t.Fatal("expected feature-disabled error")
	}
}

func TestService_FeatureFlagAllowsAccessWhenEnabled(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureCBTExams, true)

	svc := application.NewService(repo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantA, application.PermAuthor)

	q, err := svc.CreateQuestion(ctx, actor, application.CreateQuestionRequest{
		AcademicYearID: ay1,
		SubjectID:      subject1,
		QuestionText:   "Q?",
		QuestionType:   "multiple_choice",
		CorrectAnswer:  "a",
		Marks:          1,
		Options:        []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if q.ID == "" {
		t.Fatal("expected question id")
	}
}

// --- Exam lifecycle and auto-grading. ---

func TestService_ExamLifecycleAndAutoGrade(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureCBTExams, true)
	svc := application.NewService(repo, application.WithFeatureGate(gates))

	author := actorWithPerms(tenantA, application.PermAuthor)
	reader := actorWithPerms(tenantA, application.PermRead)
	student := actorStudent(tenantA, studentA, application.PermTake)
	grader := actorWithPerms(tenantA, application.PermGrade)

	q1, err := svc.CreateQuestion(ctx, author, application.CreateQuestionRequest{
		AcademicYearID: ay1, SubjectID: subject1, QuestionText: "Q1", QuestionType: "multiple_choice",
		CorrectAnswer: "a", Marks: 2, Options: []string{"a", "b", "c"},
	})
	if err != nil {
		t.Fatalf("create q1: %v", err)
	}
	q2, err := svc.CreateQuestion(ctx, author, application.CreateQuestionRequest{
		AcademicYearID: ay1, SubjectID: subject1, QuestionText: "Q2", QuestionType: "true_false",
		CorrectAnswer: "true", Marks: 1, Options: []string{"true", "false"},
	})
	if err != nil {
		t.Fatalf("create q2: %v", err)
	}

	e, err := svc.CreateExamSession(ctx, author, application.CreateExamSessionRequest{
		Title: "Midterm", AcademicYearID: ay1, SubjectID: subject1,
		QuestionIDs: []string{q1.ID, q2.ID}, DurationMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create exam: %v", err)
	}

	// Activate the exam session.
	status := string(domain.ExamStatusActive)
	_, err = svc.UpdateExamSession(ctx, author, e.ID, application.UpdateExamSessionRequest{Status: &status})
	if err != nil {
		t.Fatalf("activate exam: %v", err)
	}

	// Student starts a submission.
	sub, err := svc.StartSubmission(ctx, student, e.ID, studentA)
	if err != nil {
		t.Fatalf("start submission: %v", err)
	}
	if sub.Status != string(domain.SubmissionStatusInProgress) {
		t.Fatalf("expected in_progress, got %q", sub.Status)
	}

	// Starting a second submission should fail.
	_, err = svc.StartSubmission(ctx, student, e.ID, studentA)
	if err == nil {
		t.Fatal("expected conflict for duplicate submission")
	}

	// Submit answers: q1 correct, q2 incorrect.
	submitted, err := svc.SubmitAnswers(ctx, student, sub.ID, map[string]string{
		q1.ID: "a",
		q2.ID: "false",
	})
	if err != nil {
		t.Fatalf("submit answers: %v", err)
	}
	if submitted.Status != string(domain.SubmissionStatusSubmitted) {
		t.Fatalf("expected submitted, got %q", submitted.Status)
	}

	// Grade the submission.
	graded, err := svc.GradeSubmission(ctx, grader, sub.ID)
	if err != nil {
		t.Fatalf("grade submission: %v", err)
	}
	if graded.Status != string(domain.SubmissionStatusGraded) {
		t.Fatalf("expected graded, got %q", graded.Status)
	}
	if graded.Score == nil || *graded.Score != 2 {
		t.Fatalf("expected score 2, got %v", graded.Score)
	}
	if graded.MaxScore != 3 {
		t.Fatalf("expected max_score 3, got %d", graded.MaxScore)
	}

	// List filtered by student.
	list, _, err := svc.ListSubmissions(ctx, reader, ports.SubmissionListFilter{Limit: 10, StudentID: studentA, Status: "graded"})
	if err != nil {
		t.Fatalf("list submissions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 graded submission, got %d", len(list))
	}
}

func TestService_StartSubmissionRequiresActiveExam(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureCBTExams, true)
	svc := application.NewService(repo, application.WithFeatureGate(gates))

	author := actorWithPerms(tenantA, application.PermAuthor)
	student := actorStudent(tenantA, studentA, application.PermTake)

	q, err := svc.CreateQuestion(ctx, author, application.CreateQuestionRequest{
		AcademicYearID: ay1, SubjectID: subject1, QuestionText: "Q1", QuestionType: "multiple_choice",
		CorrectAnswer: "a", Marks: 1, Options: []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("create question: %v", err)
	}
	e, err := svc.CreateExamSession(ctx, author, application.CreateExamSessionRequest{
		Title: "Draft Exam", AcademicYearID: ay1, SubjectID: subject1,
		QuestionIDs: []string{q.ID}, DurationMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create exam: %v", err)
	}

	_, err = svc.StartSubmission(ctx, student, e.ID, studentA)
	if err == nil {
		t.Fatal("expected error starting submission for non-active exam")
	}
}

func TestService_GradeSubmissionRequiresSubmittedStatus(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureCBTExams, true)
	svc := application.NewService(repo, application.WithFeatureGate(gates))

	author := actorWithPerms(tenantA, application.PermAuthor)
	student := actorStudent(tenantA, studentA, application.PermTake)
	grader := actorWithPerms(tenantA, application.PermGrade)

	q, err := svc.CreateQuestion(ctx, author, application.CreateQuestionRequest{
		AcademicYearID: ay1, SubjectID: subject1, QuestionText: "Q1", QuestionType: "multiple_choice",
		CorrectAnswer: "a", Marks: 1, Options: []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("create question: %v", err)
	}
	e, err := svc.CreateExamSession(ctx, author, application.CreateExamSessionRequest{
		Title: "Exam", AcademicYearID: ay1, SubjectID: subject1,
		QuestionIDs: []string{q.ID}, DurationMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create exam: %v", err)
	}
	status := string(domain.ExamStatusActive)
	_, err = svc.UpdateExamSession(ctx, author, e.ID, application.UpdateExamSessionRequest{Status: &status})
	if err != nil {
		t.Fatalf("activate exam: %v", err)
	}
	sub, err := svc.StartSubmission(ctx, student, e.ID, studentA)
	if err != nil {
		t.Fatalf("start submission: %v", err)
	}

	_, err = svc.GradeSubmission(ctx, grader, sub.ID)
	if err == nil {
		t.Fatal("expected error grading in_progress submission")
	}
}
