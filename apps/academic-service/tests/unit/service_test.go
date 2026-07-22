package unit

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/auraedu/academic-service/internal/application"
	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

const (
	svcTenantA = "11111111-1111-1111-1111-111111111111"
	svcTenantB = "22222222-2222-2222-2222-222222222222"
)

// fakeDB is shared in-memory storage behind the per-aggregate fake repositories.
type fakeDB struct {
	mu       sync.Mutex
	years    map[string]*domain.AcademicYear
	terms    map[string]*domain.Term
	classes  map[string]*domain.Class
	subjects map[string]*domain.Subject
}

func newFakeDB() *fakeDB {
	return &fakeDB{
		years:    make(map[string]*domain.AcademicYear),
		terms:    make(map[string]*domain.Term),
		classes:  make(map[string]*domain.Class),
		subjects: make(map[string]*domain.Subject),
	}
}

// ---- academic years ---------------------------------------------------------

type fakeYearRepo struct{ db *fakeDB }

var _ ports.AcademicYearRepository = (*fakeYearRepo)(nil)

func (r *fakeYearRepo) Create(_ context.Context, tenantID string, y *domain.AcademicYear) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	y.TenantID = tenantID
	r.db.years[y.ID] = y
	return nil
}

func (r *fakeYearRepo) GetByID(_ context.Context, tenantID, id string) (*domain.AcademicYear, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	y, ok := r.db.years[id]
	if !ok || y.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return y, nil
}

func (r *fakeYearRepo) List(_ context.Context, tenantID string, limit int, _ string) ([]*domain.AcademicYear, string, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	var out []*domain.AcademicYear
	for _, y := range r.db.years {
		if y.TenantID == tenantID {
			out = append(out, y)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if limit > 0 && len(out) > limit {
		return out[:limit], out[limit-1].ID, nil
	}
	return out, "", nil
}

func (r *fakeYearRepo) Update(_ context.Context, tenantID string, y *domain.AcademicYear) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	existing, ok := r.db.years[y.ID]
	if !ok || existing.TenantID != tenantID {
		return domain.ErrNotFound
	}
	r.db.years[y.ID] = y
	return nil
}

func (r *fakeYearRepo) Delete(_ context.Context, tenantID, id string) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	existing, ok := r.db.years[id]
	if !ok || existing.TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(r.db.years, id)
	return nil
}

// ---- terms ------------------------------------------------------------------

type fakeTermRepo struct{ db *fakeDB }

var _ ports.TermRepository = (*fakeTermRepo)(nil)

func (r *fakeTermRepo) Create(_ context.Context, tenantID string, t *domain.Term) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	t.TenantID = tenantID
	r.db.terms[t.ID] = t
	return nil
}

func (r *fakeTermRepo) GetByID(_ context.Context, tenantID, id string) (*domain.Term, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	t, ok := r.db.terms[id]
	if !ok || t.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func (r *fakeTermRepo) List(_ context.Context, tenantID string, limit int, _ string) ([]*domain.Term, string, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	var out []*domain.Term
	for _, t := range r.db.terms {
		if t.TenantID == tenantID {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if limit > 0 && len(out) > limit {
		return out[:limit], out[limit-1].ID, nil
	}
	return out, "", nil
}

func (r *fakeTermRepo) Update(_ context.Context, tenantID string, t *domain.Term) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	existing, ok := r.db.terms[t.ID]
	if !ok || existing.TenantID != tenantID {
		return domain.ErrNotFound
	}
	r.db.terms[t.ID] = t
	return nil
}

func (r *fakeTermRepo) Delete(_ context.Context, tenantID, id string) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	existing, ok := r.db.terms[id]
	if !ok || existing.TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(r.db.terms, id)
	return nil
}

// ---- classes ----------------------------------------------------------------

type fakeClassRepo struct{ db *fakeDB }

var _ ports.ClassRepository = (*fakeClassRepo)(nil)

func (r *fakeClassRepo) Create(_ context.Context, tenantID string, c *domain.Class) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	c.TenantID = tenantID
	r.db.classes[c.ID] = c
	return nil
}

func (r *fakeClassRepo) GetByID(_ context.Context, tenantID, id string) (*domain.Class, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	c, ok := r.db.classes[id]
	if !ok || c.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (r *fakeClassRepo) List(_ context.Context, tenantID string, limit int, _ string) ([]*domain.Class, string, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	var out []*domain.Class
	for _, c := range r.db.classes {
		if c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if limit > 0 && len(out) > limit {
		return out[:limit], out[limit-1].ID, nil
	}
	return out, "", nil
}

func (r *fakeClassRepo) ListIDsByTeacher(_ context.Context, tenantID, staffID string) ([]string, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	ids := []string{}
	for _, c := range r.db.classes {
		if c.TenantID == tenantID && c.ClassTeacherID != nil && *c.ClassTeacherID == staffID {
			ids = append(ids, c.ID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (r *fakeClassRepo) Update(_ context.Context, tenantID string, c *domain.Class) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	existing, ok := r.db.classes[c.ID]
	if !ok || existing.TenantID != tenantID {
		return domain.ErrNotFound
	}
	r.db.classes[c.ID] = c
	return nil
}

func (r *fakeClassRepo) Delete(_ context.Context, tenantID, id string) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	existing, ok := r.db.classes[id]
	if !ok || existing.TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(r.db.classes, id)
	return nil
}

// ---- subjects ---------------------------------------------------------------

type fakeSubjectRepo struct{ db *fakeDB }

var _ ports.SubjectRepository = (*fakeSubjectRepo)(nil)

func (r *fakeSubjectRepo) Create(_ context.Context, tenantID string, s *domain.Subject) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	s.TenantID = tenantID
	r.db.subjects[s.ID] = s
	return nil
}

func (r *fakeSubjectRepo) GetByID(_ context.Context, tenantID, id string) (*domain.Subject, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	s, ok := r.db.subjects[id]
	if !ok || s.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return s, nil
}

func (r *fakeSubjectRepo) List(_ context.Context, tenantID string, limit int, _ string) ([]*domain.Subject, string, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	var out []*domain.Subject
	for _, s := range r.db.subjects {
		if s.TenantID == tenantID {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if limit > 0 && len(out) > limit {
		return out[:limit], out[limit-1].ID, nil
	}
	return out, "", nil
}

func (r *fakeSubjectRepo) Update(_ context.Context, tenantID string, s *domain.Subject) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	existing, ok := r.db.subjects[s.ID]
	if !ok || existing.TenantID != tenantID {
		return domain.ErrNotFound
	}
	r.db.subjects[s.ID] = s
	return nil
}

func (r *fakeSubjectRepo) Delete(_ context.Context, tenantID, id string) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()
	existing, ok := r.db.subjects[id]
	if !ok || existing.TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(r.db.subjects, id)
	return nil
}

// ---- event capture ----------------------------------------------------------

// capturedEvent records one EventPublisher call.
type capturedEvent struct {
	eventType string
	id        string
	meta      map[string]any
}

// capturePublisher is a ports.EventPublisher that records emitted events.
type capturePublisher struct {
	events []capturedEvent
}

func (p *capturePublisher) PublishYear(_ context.Context, eventType string, y *domain.AcademicYear, meta map[string]any) error {
	p.events = append(p.events, capturedEvent{eventType: eventType, id: y.ID, meta: meta})
	return nil
}

func (p *capturePublisher) PublishTerm(_ context.Context, eventType string, t *domain.Term, meta map[string]any) error {
	p.events = append(p.events, capturedEvent{eventType: eventType, id: t.ID, meta: meta})
	return nil
}

func (p *capturePublisher) PublishClass(_ context.Context, eventType string, c *domain.Class, meta map[string]any) error {
	p.events = append(p.events, capturedEvent{eventType: eventType, id: c.ID, meta: meta})
	return nil
}

func (p *capturePublisher) PublishSubject(_ context.Context, eventType string, s *domain.Subject, meta map[string]any) error {
	p.events = append(p.events, capturedEvent{eventType: eventType, id: s.ID, meta: meta})
	return nil
}

// ---- helpers ----------------------------------------------------------------

func newTestService(tenantID string) (*application.Service, *fakeDB, *capturePublisher) { //nolint:unparam // Shared helper keeps tenant intent explicit.
	db := newFakeDB()
	pub := &capturePublisher{}
	gates := flags.NewStaticSnapshot()
	gates.Set(tenantID, application.FeatureAcademicManagement, true)
	svc := application.NewService(
		&fakeYearRepo{db}, &fakeTermRepo{db}, &fakeClassRepo{db}, &fakeSubjectRepo{db},
		application.WithFeatureGate(gates),
		application.WithPublisher(pub),
	)
	return svc, db, pub
}

type fixedTeacherResolver struct {
	staffID string
	err     error
}

type fixedAssignmentResolver struct {
	staffID  string
	classIDs []string
	err      error
}

func (r fixedAssignmentResolver) ResolveTeacher(context.Context, string, string) (string, error) {
	return r.staffID, r.err
}

func (r fixedAssignmentResolver) ResolveTeacherAssignments(context.Context, string, string) (string, []string, error) {
	return r.staffID, r.classIDs, r.err
}

func (r fixedTeacherResolver) ResolveTeacher(context.Context, string, string) (string, error) {
	return r.staffID, r.err
}

func mustCreateFakeClass(t *testing.T, db *fakeDB, class *domain.Class) {
	t.Helper()
	if err := (&fakeClassRepo{db}).Create(context.Background(), svcTenantA, class); err != nil {
		t.Fatalf("create fake class: %v", err)
	}
}

func mustNewClass(t *testing.T, yearID, name string, teacherID *string) *domain.Class {
	t.Helper()
	class, err := domain.NewClass(svcTenantA, yearID, name, teacherID, nil)
	if err != nil {
		t.Fatalf("new class: %v", err)
	}
	return class
}

func TestService_ResolveTeacherClassScope(t *testing.T) {
	_, db, _ := newTestService(svcTenantA)
	teacherID := "33333333-3333-4333-8333-333333333333"
	year := seedYear(t, db, svcTenantA)
	assigned, err := domain.NewClass(svcTenantA, year.ID, "Class 1A", &teacherID, nil)
	if err != nil {
		t.Fatalf("new assigned class: %v", err)
	}
	unassignedTeacher := "44444444-4444-4444-8444-444444444444"
	unassigned, err := domain.NewClass(svcTenantA, year.ID, "Class 1B", &unassignedTeacher, nil)
	if err != nil {
		t.Fatalf("new unassigned class: %v", err)
	}
	mustCreateFakeClass(t, db, assigned)
	mustCreateFakeClass(t, db, unassigned)

	svc := application.NewService(
		&fakeYearRepo{db}, &fakeTermRepo{db}, &fakeClassRepo{db}, &fakeSubjectRepo{db},
		application.WithTeacherIdentityResolver(fixedTeacherResolver{staffID: teacherID}),
	)
	ids, err := svc.ResolveTeacherClassScope(context.Background(), svcTenantA, "user-1")
	if err != nil || len(ids) != 1 || ids[0] != assigned.ID {
		t.Fatalf("class_ids=%v err=%v", ids, err)
	}
}

func TestService_ResolveTeacherClassScopeUnionsExplicitAssignments(t *testing.T) {
	_, db, _ := newTestService(svcTenantA)
	teacherID := "33333333-3333-4333-8333-333333333333"
	year := seedYear(t, db, svcTenantA)
	owned, err := domain.NewClass(svcTenantA, year.ID, "Class 1A", &teacherID, nil)
	if err != nil {
		t.Fatal(err)
	}
	mustCreateFakeClass(t, db, owned)
	explicitID := "55555555-5555-4555-8555-555555555555"
	svc := application.NewService(
		&fakeYearRepo{db}, &fakeTermRepo{db}, &fakeClassRepo{db}, &fakeSubjectRepo{db},
		application.WithTeacherIdentityResolver(fixedAssignmentResolver{staffID: teacherID, classIDs: []string{owned.ID, explicitID}}),
	)
	ids, err := svc.ResolveTeacherClassScope(context.Background(), svcTenantA, "user-1")
	if err != nil || len(ids) != 2 || ids[0] != owned.ID || ids[1] != explicitID {
		t.Fatalf("class_ids=%v err=%v", ids, err)
	}
}

func TestService_ResolveTeacherClassScopeFailsClosed(t *testing.T) {
	svc, _, _ := newTestService(svcTenantA)
	if _, err := svc.ResolveTeacherClassScope(context.Background(), svcTenantA, "user-1"); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("unconfigured=%v", err)
	}
}

func TestService_TeacherClassReadsAreAssignedOnly(t *testing.T) {
	_, db, _ := newTestService(svcTenantA)
	teacherID := "33333333-3333-4333-8333-333333333333"
	year := seedYear(t, db, svcTenantA)
	assigned := mustNewClass(t, year.ID, "Class 1A", &teacherID)
	otherTeacher := "44444444-4444-4444-8444-444444444444"
	other := mustNewClass(t, year.ID, "Class 1B", &otherTeacher)
	mustCreateFakeClass(t, db, assigned)
	mustCreateFakeClass(t, db, other)
	gates := flags.NewStaticSnapshot()
	gates.Set(svcTenantA, application.FeatureAcademicManagement, true)
	svc := application.NewService(
		&fakeYearRepo{db}, &fakeTermRepo{db}, &fakeClassRepo{db}, &fakeSubjectRepo{db},
		application.WithFeatureGate(gates),
		application.WithTeacherIdentityResolver(fixedTeacherResolver{staffID: teacherID}),
	)
	teacher := svcActor(svcTenantA, application.PermRead)
	teacher.Role = "teacher"
	classes, _, err := svc.ListClasses(svcCtx(svcTenantA), teacher, 20, "")
	if err != nil || len(classes) != 1 || classes[0].ID != assigned.ID {
		t.Fatalf("classes=%+v err=%v", classes, err)
	}
	if _, err := svc.GetClass(svcCtx(svcTenantA), teacher, other.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unassigned get=%v", err)
	}
}

func TestService_LearnerClassReadsAreLinkedOnly(t *testing.T) {
	_, db, _ := newTestService(svcTenantA)
	year := seedYear(t, db, svcTenantA)
	linked := mustNewClass(t, year.ID, "Class 1A", nil)
	other := mustNewClass(t, year.ID, "Class 1B", nil)
	mustCreateFakeClass(t, db, linked)
	mustCreateFakeClass(t, db, other)
	gates := flags.NewStaticSnapshot()
	gates.Set(svcTenantA, application.FeatureAcademicManagement, true)
	svc := application.NewService(
		&fakeYearRepo{db}, &fakeTermRepo{db}, &fakeClassRepo{db}, &fakeSubjectRepo{db},
		application.WithFeatureGate(gates),
		application.WithLearnerScopeResolver(classScopeStub{ids: []string{linked.ID}}),
	)

	for _, role := range []string{"student", "parent"} {
		t.Run(role, func(t *testing.T) {
			actor := svcActor(svcTenantA, application.PermRead)
			actor.Role = role
			classes, _, err := svc.ListClasses(svcCtx(svcTenantA), actor, 20, "")
			if err != nil || len(classes) != 1 || classes[0].ID != linked.ID {
				t.Fatalf("classes=%+v err=%v", classes, err)
			}
			if _, err := svc.GetClass(svcCtx(svcTenantA), actor, other.ID); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("unlinked get=%v", err)
			}
		})
	}
}

func TestService_LearnerClassReadsFailClosedWithoutResolver(t *testing.T) {
	svc, _, _ := newTestService(svcTenantA)
	actor := svcActor(svcTenantA, application.PermRead)
	actor.Role = "parent"
	if _, _, err := svc.ListClasses(svcCtx(svcTenantA), actor, 20, ""); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("list error=%v", err)
	}
	if _, err := svc.GetClass(svcCtx(svcTenantA), actor, "class-1"); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("get error=%v", err)
	}
}

func svcCtx(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

func svcActor(tenantID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantID, Permissions: perms}
}

// seedYear inserts an academic year directly into the fake repo (bypassing use cases).
func seedYear(t *testing.T, db *fakeDB, tenantID string) *domain.AcademicYear { //nolint:unparam // Shared helper keeps tenant intent explicit.
	t.Helper()
	y, err := domain.NewAcademicYear(tenantID, "2025/26", "", "2025-09-01", "2026-07-31", false)
	if err != nil {
		t.Fatalf("new academic year: %v", err)
	}
	if err := (&fakeYearRepo{db}).Create(context.Background(), tenantID, y); err != nil {
		t.Fatalf("create academic year: %v", err)
	}
	return y
}

// ---- term use cases ---------------------------------------------------------

func TestService_Term_CreateListGetUpdateDelete(t *testing.T) {
	svc, db, _ := newTestService(svcTenantA)
	ctx := svcCtx(svcTenantA)
	actor := svcActor(svcTenantA, application.PermRead, application.PermManage)
	year := seedYear(t, db, svcTenantA)

	term, err := svc.CreateTerm(ctx, actor, application.CreateTermRequest{
		AcademicYearID: year.ID,
		Name:           "Term 1",
		StartDate:      "2025-09-01",
		EndDate:        "2025-12-31",
	})
	if err != nil {
		t.Fatalf("create term: %v", err)
	}
	if term.TenantID != svcTenantA || term.AcademicYearID != year.ID {
		t.Fatalf("term not tenant/year scoped: %+v", term)
	}

	got, err := svc.GetTerm(ctx, actor, term.ID)
	if err != nil {
		t.Fatalf("get term: %v", err)
	}
	if got.ID != term.ID || got.Name != "Term 1" {
		t.Fatalf("term mismatch: %+v", got)
	}

	list, _, err := svc.ListTerms(ctx, actor, 25, "")
	if err != nil {
		t.Fatalf("list terms: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 term, got %d", len(list))
	}

	name := "First Term"
	updated, err := svc.UpdateTerm(ctx, actor, term.ID, application.UpdateTermRequest{Name: &name})
	if err != nil {
		t.Fatalf("update term: %v", err)
	}
	if updated.Name != name {
		t.Fatalf("term not updated: %+v", updated)
	}

	if err := svc.DeleteTerm(ctx, actor, term.ID); err != nil {
		t.Fatalf("delete term: %v", err)
	}
	if _, err := svc.GetTerm(ctx, actor, term.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func TestService_CreateTerm_UnknownAcademicYear(t *testing.T) {
	svc, _, _ := newTestService(svcTenantA)
	ctx := svcCtx(svcTenantA)
	actor := svcActor(svcTenantA, application.PermManage)

	_, err := svc.CreateTerm(ctx, actor, application.CreateTermRequest{
		AcademicYearID: "00000000-0000-0000-0000-000000000000",
		Name:           "Term 1",
		StartDate:      "2025-09-01",
		EndDate:        "2025-12-31",
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for unknown academic year, got %v", err)
	}
}

func TestService_CreateTerm_CrossTenantAcademicYear(t *testing.T) {
	_, db, _ := newTestService(svcTenantA)
	year := seedYear(t, db, svcTenantA)

	// Tenant B must not attach a term to tenant A's academic year.
	gates := flags.NewStaticSnapshot()
	gates.Set(svcTenantB, application.FeatureAcademicManagement, true)
	svc := application.NewService(
		&fakeYearRepo{db}, &fakeTermRepo{db}, &fakeClassRepo{db}, &fakeSubjectRepo{db},
		application.WithFeatureGate(gates),
	)
	ctx := svcCtx(svcTenantB)
	actor := svcActor(svcTenantB, application.PermManage)
	_, err := svc.CreateTerm(ctx, actor, application.CreateTermRequest{
		AcademicYearID: year.ID,
		Name:           "Term 1",
		StartDate:      "2025-09-01",
		EndDate:        "2025-12-31",
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for cross-tenant academic year, got %v", err)
	}
}

func TestService_TermDatesMustRemainWithinAcademicYear(t *testing.T) {
	svc, db, _ := newTestService(svcTenantA)
	ctx := svcCtx(svcTenantA)
	actor := svcActor(svcTenantA, application.PermManage)
	year := seedYear(t, db, svcTenantA)

	for _, dates := range []struct {
		name  string
		start string
		end   string
	}{
		{name: "starts before year", start: "2025-08-31", end: "2025-12-20"},
		{name: "ends after year", start: "2026-04-01", end: "2026-08-01"},
	} {
		t.Run(dates.name, func(t *testing.T) {
			_, err := svc.CreateTerm(ctx, actor, application.CreateTermRequest{
				AcademicYearID: year.ID,
				Name:           "Boundary term",
				StartDate:      dates.start,
				EndDate:        dates.end,
			})
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("expected year-boundary validation, got %v", err)
			}
		})
	}

	term, err := svc.CreateTerm(ctx, actor, application.CreateTermRequest{
		AcademicYearID: year.ID,
		Name:           "Term 3",
		StartDate:      "2026-04-01",
		EndDate:        "2026-07-15",
	})
	if err != nil {
		t.Fatalf("create valid term: %v", err)
	}
	outOfBounds := "2026-08-01"
	if _, err := svc.UpdateTerm(ctx, actor, term.ID, application.UpdateTermRequest{EndDate: &outOfBounds}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected update boundary validation, got %v", err)
	}
}

// ---- class use cases --------------------------------------------------------

func TestService_Class_CreateListGetUpdateDelete(t *testing.T) {
	svc, db, _ := newTestService(svcTenantA)
	ctx := svcCtx(svcTenantA)
	actor := svcActor(svcTenantA, application.PermRead, application.PermManage)
	year := seedYear(t, db, svcTenantA)

	capacity := 45
	class, err := svc.CreateClass(ctx, actor, application.CreateClassRequest{
		Name:           "Class 1",
		AcademicYearID: year.ID,
		Capacity:       &capacity,
	})
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	if class.TenantID != svcTenantA || class.AcademicYearID != year.ID {
		t.Fatalf("class not tenant/year scoped: %+v", class)
	}

	got, err := svc.GetClass(ctx, actor, class.ID)
	if err != nil {
		t.Fatalf("get class: %v", err)
	}
	if got.ID != class.ID || got.Capacity == nil || *got.Capacity != capacity {
		t.Fatalf("class mismatch: %+v", got)
	}

	list, _, err := svc.ListClasses(ctx, actor, 25, "")
	if err != nil {
		t.Fatalf("list classes: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 class, got %d", len(list))
	}

	name := "Class 1A"
	updated, err := svc.UpdateClass(ctx, actor, class.ID, application.UpdateClassRequest{Name: &name})
	if err != nil {
		t.Fatalf("update class: %v", err)
	}
	if updated.Name != name {
		t.Fatalf("class not updated: %+v", updated)
	}

	if err := svc.DeleteClass(ctx, actor, class.ID); err != nil {
		t.Fatalf("delete class: %v", err)
	}
	if _, err := svc.GetClass(ctx, actor, class.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func TestService_CreateClass_UnknownAcademicYear(t *testing.T) {
	svc, _, _ := newTestService(svcTenantA)
	ctx := svcCtx(svcTenantA)
	actor := svcActor(svcTenantA, application.PermManage)

	_, err := svc.CreateClass(ctx, actor, application.CreateClassRequest{
		Name:           "Class 1",
		AcademicYearID: "00000000-0000-0000-0000-000000000000",
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for unknown academic year, got %v", err)
	}
}

// ---- subject use cases ------------------------------------------------------

func TestService_Subject_CreateListGetUpdateDelete(t *testing.T) {
	svc, _, _ := newTestService(svcTenantA)
	ctx := svcCtx(svcTenantA)
	actor := svcActor(svcTenantA, application.PermRead, application.PermManage)

	code := "MATH"
	subject, err := svc.CreateSubject(ctx, actor, application.CreateSubjectRequest{
		Name: "Mathematics",
		Code: &code,
	})
	if err != nil {
		t.Fatalf("create subject: %v", err)
	}
	if subject.TenantID != svcTenantA {
		t.Fatalf("subject not tenant scoped: %+v", subject)
	}

	got, err := svc.GetSubject(ctx, actor, subject.ID)
	if err != nil {
		t.Fatalf("get subject: %v", err)
	}
	if got.ID != subject.ID || got.Code == nil || *got.Code != code {
		t.Fatalf("subject mismatch: %+v", got)
	}

	list, _, err := svc.ListSubjects(ctx, actor, 25, "")
	if err != nil {
		t.Fatalf("list subjects: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(list))
	}

	name := "Core Mathematics"
	updated, err := svc.UpdateSubject(ctx, actor, subject.ID, application.UpdateSubjectRequest{Name: &name})
	if err != nil {
		t.Fatalf("update subject: %v", err)
	}
	if updated.Name != name {
		t.Fatalf("subject not updated: %+v", updated)
	}

	if err := svc.DeleteSubject(ctx, actor, subject.ID); err != nil {
		t.Fatalf("delete subject: %v", err)
	}
	if _, err := svc.GetSubject(ctx, actor, subject.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

// ---- cross-cutting gates ----------------------------------------------------

func TestService_TenantIsolation(t *testing.T) {
	db := newFakeDB()
	gates := flags.NewStaticSnapshot()
	gates.Set(svcTenantA, application.FeatureAcademicManagement, true)
	gates.Set(svcTenantB, application.FeatureAcademicManagement, true)
	svc := application.NewService(
		&fakeYearRepo{db}, &fakeTermRepo{db}, &fakeClassRepo{db}, &fakeSubjectRepo{db},
		application.WithFeatureGate(gates),
	)

	aCtx := svcCtx(svcTenantA)
	aActor := svcActor(svcTenantA, application.PermRead, application.PermManage)
	year := seedYear(t, db, svcTenantA)

	term, err := svc.CreateTerm(aCtx, aActor, application.CreateTermRequest{
		AcademicYearID: year.ID, Name: "Term 1", StartDate: "2025-09-01", EndDate: "2025-12-31",
	})
	if err != nil {
		t.Fatalf("create term: %v", err)
	}
	class, err := svc.CreateClass(aCtx, aActor, application.CreateClassRequest{Name: "Class 1", AcademicYearID: year.ID})
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	subject, err := svc.CreateSubject(aCtx, aActor, application.CreateSubjectRequest{Name: "Mathematics"})
	if err != nil {
		t.Fatalf("create subject: %v", err)
	}

	bCtx := svcCtx(svcTenantB)
	bActor := svcActor(svcTenantB, application.PermRead, application.PermManage)

	if _, err := svc.GetTerm(bCtx, bActor, term.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tenant B should not read tenant A term, got %v", err)
	}
	if _, err := svc.GetClass(bCtx, bActor, class.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tenant B should not read tenant A class, got %v", err)
	}
	if _, err := svc.GetSubject(bCtx, bActor, subject.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tenant B should not read tenant A subject, got %v", err)
	}
	if err := svc.DeleteSubject(bCtx, bActor, subject.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tenant B should not delete tenant A subject, got %v", err)
	}
	if list, _, err := svc.ListSubjects(bCtx, bActor, 25, ""); err != nil || len(list) != 0 {
		t.Fatalf("tenant B should see 0 subjects, got %d (%v)", len(list), err)
	}
}

func TestService_ForbiddenWithoutPermission(t *testing.T) {
	svc, db, _ := newTestService(svcTenantA)
	ctx := svcCtx(svcTenantA)
	year := seedYear(t, db, svcTenantA)

	// academic.read holder may not mutate.
	reader := svcActor(svcTenantA, application.PermRead)
	if _, err := svc.CreateSubject(ctx, reader, application.CreateSubjectRequest{Name: "Mathematics"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden for mutation without academic.manage, got %v", err)
	}
	if _, err := svc.CreateTerm(ctx, reader, application.CreateTermRequest{
		AcademicYearID: year.ID, Name: "Term 1", StartDate: "2025-09-01", EndDate: "2025-12-31",
	}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden for term create without academic.manage, got %v", err)
	}

	// Unauthenticated actor may not read.
	if _, _, err := svc.ListSubjects(ctx, auth.Actor{}, 25, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden for unauthenticated read, got %v", err)
	}
}

func TestService_TenantMismatchForbidden(t *testing.T) {
	svc, _, _ := newTestService(svcTenantA)
	// Actor belongs to tenant B but the request context carries tenant A.
	ctx := svcCtx(svcTenantA)
	actor := svcActor(svcTenantB, application.PermRead)
	if _, _, err := svc.ListSubjects(ctx, actor, 25, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden for cross-tenant actor, got %v", err)
	}
}

func TestService_FeatureDisabled(t *testing.T) {
	db := newFakeDB()
	gates := flags.NewStaticSnapshot() // all features disabled
	svc := application.NewService(
		&fakeYearRepo{db}, &fakeTermRepo{db}, &fakeClassRepo{db}, &fakeSubjectRepo{db},
		application.WithFeatureGate(gates),
	)
	ctx := svcCtx(svcTenantA)
	actor := svcActor(svcTenantA, application.PermRead, application.PermManage)

	if _, _, err := svc.ListSubjects(ctx, actor, 25, ""); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("expected feature disabled on list, got %v", err)
	}
	if _, err := svc.CreateSubject(ctx, actor, application.CreateSubjectRequest{Name: "Mathematics"}); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("expected feature disabled on create subject, got %v", err)
	}
	if _, err := svc.CreateClass(ctx, actor, application.CreateClassRequest{Name: "Class 1", AcademicYearID: "ay-1"}); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("expected feature disabled on create class, got %v", err)
	}
	if _, err := svc.CreateTerm(ctx, actor, application.CreateTermRequest{
		AcademicYearID: "ay-1", Name: "Term 1", StartDate: "2025-09-01", EndDate: "2025-12-31",
	}); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("expected feature disabled on create term, got %v", err)
	}
}

// ---- event emission ---------------------------------------------------------

func TestService_CreateClass_PublishesContractedEvent(t *testing.T) {
	svc, db, pub := newTestService(svcTenantA)
	ctx := svcCtx(svcTenantA)
	actor := svcActor(svcTenantA, application.PermManage)
	year := seedYear(t, db, svcTenantA)

	class, err := svc.CreateClass(ctx, actor, application.CreateClassRequest{Name: "Class 1", AcademicYearID: year.ID})
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected exactly 1 event, got %d", len(pub.events))
	}
	got := pub.events[0]
	if got.eventType != "academic.class_created.v1" {
		t.Fatalf("expected academic.class_created.v1 (contracts/events/academic.class_created.v1.json), got %q", got.eventType)
	}
	if got.id != class.ID {
		t.Fatalf("event not bound to created class: %+v", got)
	}
}

func TestService_CreateSubject_PublishesContractedEvent(t *testing.T) {
	svc, _, pub := newTestService(svcTenantA)
	ctx := svcCtx(svcTenantA)
	actor := svcActor(svcTenantA, application.PermManage)

	subject, err := svc.CreateSubject(ctx, actor, application.CreateSubjectRequest{Name: "Mathematics"})
	if err != nil {
		t.Fatalf("create subject: %v", err)
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected exactly 1 event, got %d", len(pub.events))
	}
	got := pub.events[0]
	if got.eventType != "academic.subject_created.v1" {
		t.Fatalf("expected academic.subject_created.v1 (contracts/events/academic.subject_created.v1.json), got %q", got.eventType)
	}
	if got.id != subject.ID {
		t.Fatalf("event not bound to created subject: %+v", got)
	}
}

// TestService_CreateTerm_PublishesNothing pins the decision that no term_created event
// is emitted: contracts/events/ has no academic.term_created schema, and only
// contracted created events are emitted (the years precedent covers updated/deleted).
func TestService_CreateTerm_PublishesNothing(t *testing.T) {
	svc, db, pub := newTestService(svcTenantA)
	ctx := svcCtx(svcTenantA)
	actor := svcActor(svcTenantA, application.PermManage)
	year := seedYear(t, db, svcTenantA)

	if _, err := svc.CreateTerm(ctx, actor, application.CreateTermRequest{
		AcademicYearID: year.ID, Name: "Term 1", StartDate: "2025-09-01", EndDate: "2025-12-31",
	}); err != nil {
		t.Fatalf("create term: %v", err)
	}
	if len(pub.events) != 0 {
		t.Fatalf("expected no events (no academic.term_created contract exists), got %d", len(pub.events))
	}
}

func TestService_Update_PublishesChangedFields(t *testing.T) {
	svc, _, pub := newTestService(svcTenantA)
	ctx := svcCtx(svcTenantA)
	actor := svcActor(svcTenantA, application.PermManage)

	subject, err := svc.CreateSubject(ctx, actor, application.CreateSubjectRequest{Name: "Mathematics"})
	if err != nil {
		t.Fatalf("create subject: %v", err)
	}
	pub.events = nil

	name := "Core Mathematics"
	if _, err := svc.UpdateSubject(ctx, actor, subject.ID, application.UpdateSubjectRequest{Name: &name}); err != nil {
		t.Fatalf("update subject: %v", err)
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected exactly 1 event, got %d", len(pub.events))
	}
	got := pub.events[0]
	if got.eventType != "academic.subject_updated.v1" {
		t.Fatalf("expected academic.subject_updated.v1, got %q", got.eventType)
	}
	changed, ok := got.meta["changed_fields"].([]string)
	if !ok || len(changed) != 1 || changed[0] != "name" {
		t.Errorf("changed_fields: expected [name], got %v", got.meta["changed_fields"])
	}
}

func TestService_Update_NoChange_PublishesNothing(t *testing.T) {
	svc, _, pub := newTestService(svcTenantA)
	ctx := svcCtx(svcTenantA)
	actor := svcActor(svcTenantA, application.PermManage)

	subject, err := svc.CreateSubject(ctx, actor, application.CreateSubjectRequest{Name: "Mathematics"})
	if err != nil {
		t.Fatalf("create subject: %v", err)
	}
	pub.events = nil

	if _, err := svc.UpdateSubject(ctx, actor, subject.ID, application.UpdateSubjectRequest{}); err != nil {
		t.Fatalf("update subject: %v", err)
	}
	if len(pub.events) != 0 {
		t.Fatalf("expected no events for a no-op update, got %d", len(pub.events))
	}
}

func TestService_Delete_PublishesEvent(t *testing.T) {
	svc, _, pub := newTestService(svcTenantA)
	ctx := svcCtx(svcTenantA)
	actor := svcActor(svcTenantA, application.PermManage)

	subject, err := svc.CreateSubject(ctx, actor, application.CreateSubjectRequest{Name: "Mathematics"})
	if err != nil {
		t.Fatalf("create subject: %v", err)
	}
	pub.events = nil

	if err := svc.DeleteSubject(ctx, actor, subject.ID); err != nil {
		t.Fatalf("delete subject: %v", err)
	}
	if len(pub.events) != 1 || pub.events[0].eventType != "academic.subject_deleted.v1" {
		t.Fatalf("expected academic.subject_deleted.v1, got %+v", pub.events)
	}
}
