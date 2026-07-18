package unit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/auraedu/assessment-service/internal/application"
	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/assessment-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

const tenantB = "22222222-2222-2222-2222-222222222222"

// --- In-memory fakes. ---

type fakeRepo struct {
	mu          sync.Mutex
	assessments map[string]*domain.Assessment
	scores      map[string]*domain.Score
	gradeRows   []domain.GradeRow
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		assessments: make(map[string]*domain.Assessment),
		scores:      make(map[string]*domain.Score),
	}
}

func key(tenantID, id string) string { return tenantID + "/" + id }

func (f *fakeRepo) CreateAssessment(_ context.Context, tenantID string, a *domain.Assessment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.assessments[key(tenantID, a.ID)] = a
	return nil
}

func (f *fakeRepo) GetAssessmentByID(_ context.Context, tenantID, id string) (*domain.Assessment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.assessments[key(tenantID, id)]
	if !ok || a.DeletedAt != nil {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

func (f *fakeRepo) ListAssessments(_ context.Context, tenantID string, filter ports.AssessmentListFilter) ([]*domain.Assessment, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Assessment
	for _, a := range f.assessments {
		if a.TenantID == tenantID && a.DeletedAt == nil {
			out = append(out, a)
		}
	}
	return out, "", nil
}

func (f *fakeRepo) UpdateAssessment(_ context.Context, tenantID string, a *domain.Assessment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.assessments[key(tenantID, a.ID)]; !ok {
		return domain.ErrNotFound
	}
	f.assessments[key(tenantID, a.ID)] = a
	return nil
}

func (f *fakeRepo) DeleteAssessment(_ context.Context, tenantID, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.assessments[key(tenantID, id)]
	if !ok {
		return domain.ErrNotFound
	}
	now := time.Now().UTC()
	a.DeletedAt = &now
	return nil
}

func (f *fakeRepo) CreateScore(_ context.Context, tenantID string, s *domain.Score) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scores[key(tenantID, s.ID)] = s
	return nil
}

func (f *fakeRepo) GetScoreByID(_ context.Context, tenantID, assessmentID, scoreID string) (*domain.Score, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.scores[key(tenantID, scoreID)]
	if !ok || s.AssessmentID != assessmentID || s.DeletedAt != nil {
		return nil, domain.ErrNotFound
	}
	return s, nil
}

func (f *fakeRepo) ListScores(_ context.Context, tenantID, assessmentID string, _ ports.ScoreListFilter) ([]*domain.Score, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Score
	for _, s := range f.scores {
		if s.TenantID == tenantID && s.AssessmentID == assessmentID && s.DeletedAt == nil {
			out = append(out, s)
		}
	}
	return out, "", nil
}

func (f *fakeRepo) UpdateScore(_ context.Context, tenantID string, s *domain.Score) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scores[key(tenantID, s.ID)] = s
	return nil
}

func (f *fakeRepo) DeleteScore(_ context.Context, tenantID, assessmentID, scoreID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.scores, key(tenantID, scoreID))
	return nil
}

func (f *fakeRepo) ListAssignments(_ context.Context, tenantID string, filter ports.AssignmentListFilter) ([]*domain.Assessment, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Assessment
	for _, a := range f.assessments {
		if a.TenantID != tenantID || a.DeletedAt != nil || !a.IsAssignment() {
			continue
		}
		if filter.SubjectID != "" && a.SubjectID != filter.SubjectID {
			continue
		}
		if filter.Status != "" && a.Status != filter.Status {
			continue
		}
		if filter.ClassID != "" && !contains(a.ClassIDs, filter.ClassID) {
			continue
		}
		out = append(out, a)
	}
	return out, "", nil
}

func (f *fakeRepo) GradebookScores(_ context.Context, tenantID string, _ ports.GradebookFilter) ([]domain.GradeRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.GradeRow(nil), f.gradeRows...), nil
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

type publishedEvent struct {
	eventType  string
	assignment *domain.Assessment
}

type fakePublisher struct {
	mu       sync.Mutex
	events   []publishedEvent
	assessEv []string
}

func (p *fakePublisher) PublishAssessment(_ context.Context, eventType string, _ *domain.Assessment, _ map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.assessEv = append(p.assessEv, eventType)
	return nil
}

func (p *fakePublisher) PublishAssignment(_ context.Context, eventType string, a *domain.Assessment, _ map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, publishedEvent{eventType: eventType, assignment: a})
	return nil
}

func (p *fakePublisher) PublishScore(_ context.Context, _ string, _ *domain.Score, _ map[string]any) error {
	return nil
}

// --- Helpers. ---

func svcWithGates(repo ports.Repository, pub ports.EventPublisher, gates *flags.StaticSnapshot) *application.Service {
	opts := []application.Option{application.WithFeatureGate(gates)}
	if pub != nil {
		opts = append(opts, application.WithPublisher(pub))
	}
	return application.NewService(repo, opts...)
}

func enabledGates(tenantID string) *flags.StaticSnapshot {
	g := flags.NewStaticSnapshot()
	g.Set(tenantID, application.FeatureAssessments, true)
	g.Set(tenantID, application.FeatureAssignments, true)
	return g
}

func ctxFor(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

func actor(tenantID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantID, Permissions: perms}
}

func createReq() application.CreateAssignmentRequest {
	return application.CreateAssignmentRequest{
		AcademicYearID: ay1,
		SubjectID:      subject1,
		Title:          "Essay 1",
		Instructions:   "Write 500 words",
		MaxScore:       50,
		ClassIDs:       []string{class1},
	}
}

// --- Assignment use case tests. ---

func TestService_AssignmentCRUDAndPublish(t *testing.T) {
	repo := newFakeRepo()
	pub := &fakePublisher{}
	svc := svcWithGates(repo, pub, enabledGates(tenantA))
	ctx := ctxFor(tenantA)
	manager := actor(tenantA, application.PermManage)
	reader := actor(tenantA, application.PermRead)

	a, err := svc.CreateAssignment(ctx, manager, createReq())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.Type != string(domain.TypeAssignment) || a.Status != string(domain.StatusDraft) {
		t.Fatalf("unexpected assignment: %+v", a)
	}

	got, err := svc.GetAssignment(ctx, reader, a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != a.ID || got.Title != "Essay 1" {
		t.Fatalf("get mismatch: %+v", got)
	}

	list, _, err := svc.ListAssignments(ctx, reader, ports.AssignmentListFilter{ClassID: class1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 assignment for class1, got %d", len(list))
	}
	list, _, err = svc.ListAssignments(ctx, reader, ports.AssignmentListFilter{ClassID: class2})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 assignments for class2, got %d", len(list))
	}

	title := "Essay 1 (revised)"
	updated, err := svc.UpdateAssignment(ctx, manager, a.ID, application.UpdateAssignmentRequest{Title: &title})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Title != title {
		t.Fatalf("update mismatch: %+v", updated)
	}

	published, err := svc.PublishAssignment(ctx, manager, a.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Status != string(domain.StatusPublished) || published.PublishedAt == nil {
		t.Fatalf("expected published assignment, got %+v", published)
	}
	if len(pub.events) != 1 || pub.events[0].eventType != "assignment.published.v1" {
		t.Fatalf("expected assignment.published.v1 event, got %+v", pub.events)
	}
	if pub.events[0].assignment.ID != a.ID {
		t.Fatalf("event carries wrong assignment: %+v", pub.events[0])
	}

	if _, err := svc.PublishAssignment(ctx, manager, a.ID); err == nil {
		t.Fatal("expected error publishing twice")
	}

	if err := svc.DeleteAssignment(ctx, manager, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.GetAssignment(ctx, reader, a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func TestService_GetAssignmentRejectsNonAssignment(t *testing.T) {
	repo := newFakeRepo()
	svc := svcWithGates(repo, nil, enabledGates(tenantA))
	ctx := ctxFor(tenantA)
	manager := actor(tenantA, application.PermManage)

	test, err := svc.CreateAssessment(ctx, manager, application.CreateAssessmentRequest{
		AcademicYearID: ay1, SubjectID: subject1, Type: "test", Title: "Midterm", MaxScore: 100,
	})
	if err != nil {
		t.Fatalf("create assessment: %v", err)
	}
	if _, err := svc.GetAssignment(ctx, actor(tenantA, application.PermRead), test.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for a test via the assignments API, got %v", err)
	}
	if err := svc.DeleteAssignment(ctx, manager, test.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found deleting a test via the assignments API, got %v", err)
	}
}

// --- Feature-flag gating (both flags). ---

func TestService_AssignmentsFlagGatesAssignmentEndpoints(t *testing.T) {
	repo := newFakeRepo()
	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureAssessments, true)
	gates.Set(tenantA, application.FeatureAssignments, false)
	svc := svcWithGates(repo, nil, gates)
	ctx := ctxFor(tenantA)
	manager := actor(tenantA, application.PermManage)
	reader := actor(tenantA, application.PermRead)

	if _, err := svc.CreateAssignment(ctx, manager, createReq()); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("create: expected feature-disabled, got %v", err)
	}
	if _, _, err := svc.ListAssignments(ctx, reader, ports.AssignmentListFilter{}); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("list: expected feature-disabled, got %v", err)
	}
	if _, err := svc.GetAssignment(ctx, reader, "any"); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("get: expected feature-disabled, got %v", err)
	}
	if _, err := svc.UpdateAssignment(ctx, manager, "any", application.UpdateAssignmentRequest{}); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("update: expected feature-disabled, got %v", err)
	}
	if err := svc.DeleteAssignment(ctx, manager, "any"); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("delete: expected feature-disabled, got %v", err)
	}
	if _, err := svc.PublishAssignment(ctx, manager, "any"); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("publish: expected feature-disabled, got %v", err)
	}
}

func TestService_AssessmentsFlagGatesAssessmentsAndGradebook(t *testing.T) {
	repo := newFakeRepo()
	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureAssessments, false)
	gates.Set(tenantA, application.FeatureAssignments, true)
	svc := svcWithGates(repo, nil, gates)
	ctx := ctxFor(tenantA)
	manager := actor(tenantA, application.PermManage)
	reader := actor(tenantA, application.PermRead)

	if _, err := svc.CreateAssessment(ctx, manager, application.CreateAssessmentRequest{
		AcademicYearID: ay1, SubjectID: subject1, Type: "test", Title: "Midterm", MaxScore: 100,
	}); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("assessments: expected feature-disabled, got %v", err)
	}
	if _, err := svc.GetGradebook(ctx, reader, ports.GradebookFilter{StudentID: student1}); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("gradebook: expected feature-disabled, got %v", err)
	}

	// The assignments flag is independent: assignment endpoints still work.
	if _, err := svc.CreateAssignment(ctx, manager, createReq()); err != nil {
		t.Fatalf("assignments should be gated only on the assignments flag: %v", err)
	}
}

// --- RBAC + tenant scoping. ---

func TestService_AssignmentRequiresManagePermission(t *testing.T) {
	repo := newFakeRepo()
	svc := svcWithGates(repo, nil, enabledGates(tenantA))
	ctx := ctxFor(tenantA)
	reader := actor(tenantA, application.PermRead)

	if _, err := svc.CreateAssignment(ctx, reader, createReq()); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden without assessments.manage, got %v", err)
	}
}

func TestService_AssignmentTenantScoping(t *testing.T) {
	repo := newFakeRepo()
	gates := enabledGates(tenantA)
	gates.Set(tenantB, application.FeatureAssessments, true)
	gates.Set(tenantB, application.FeatureAssignments, true)
	svc := svcWithGates(repo, nil, gates)
	ctxA := ctxFor(tenantA)
	managerA := actor(tenantA, application.PermManage)

	a, err := svc.CreateAssignment(ctxA, managerA, createReq())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// An actor from tenant B presented with tenant A's context is rejected.
	actorB := actor(tenantB, application.PermManage, application.PermRead)
	if _, err := svc.GetAssignment(ctxA, actorB, a.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden for cross-tenant actor, got %v", err)
	}

	// Tenant B in its own context cannot see tenant A's assignment.
	ctxB := ctxFor(tenantB)
	if _, err := svc.GetAssignment(ctxB, actorB, a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for tenant B, got %v", err)
	}
	list, _, err := svc.ListAssignments(ctxB, actorB, ports.AssignmentListFilter{})
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 assignments, got %d", len(list))
	}
}

// --- Gradebook through the service. ---

func TestService_GetGradebook(t *testing.T) {
	repo := newFakeRepo()
	repo.gradeRows = []domain.GradeRow{
		{SubjectID: subject1, Score: 90, MaxScore: 100},
		{SubjectID: subject2, Score: 40, MaxScore: 50},
	}
	svc := svcWithGates(repo, nil, enabledGates(tenantA))
	ctx := ctxFor(tenantA)
	reader := actor(tenantA, application.PermRead)

	book, err := svc.GetGradebook(ctx, reader, ports.GradebookFilter{StudentID: student1, AcademicYearID: ay1})
	if err != nil {
		t.Fatalf("gradebook: %v", err)
	}
	if book.StudentID != student1 || book.AcademicYearID != ay1 {
		t.Fatalf("filter not echoed: %+v", book)
	}
	if len(book.Subjects) != 2 {
		t.Fatalf("expected 2 subjects, got %d", len(book.Subjects))
	}
	if book.Overall.AssessmentCount != 2 || book.Overall.TotalScore != 130 || book.Overall.TotalMaxScore != 150 {
		t.Fatalf("wrong overall: %+v", book.Overall)
	}
}

func TestService_GetGradebook_Empty(t *testing.T) {
	repo := newFakeRepo()
	svc := svcWithGates(repo, nil, enabledGates(tenantA))
	book, err := svc.GetGradebook(ctxFor(tenantA), actor(tenantA, application.PermRead), ports.GradebookFilter{ClassID: class1})
	if err != nil {
		t.Fatalf("gradebook: %v", err)
	}
	if len(book.Subjects) != 0 || book.Overall.Average != nil || book.Overall.WeightedAverage != nil {
		t.Fatalf("expected empty gradebook with nil averages, got %+v", book)
	}
	if book.ClassID != class1 {
		t.Fatalf("class filter not echoed: %+v", book)
	}
}

func TestService_GetGradebook_RequiresStudentOrClass(t *testing.T) {
	repo := newFakeRepo()
	svc := svcWithGates(repo, nil, enabledGates(tenantA))
	_, err := svc.GetGradebook(ctxFor(tenantA), actor(tenantA, application.PermRead), ports.GradebookFilter{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error when neither student_id nor class_id given, got %v", err)
	}
}
