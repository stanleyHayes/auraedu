package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/academic-service/internal/application"
	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
)

type fakeTimetableRepo struct {
	entries map[string]*domain.TimetableEntry
}

func (r *fakeTimetableRepo) Create(_ context.Context, _ string, e *domain.TimetableEntry) error {
	r.entries[e.ID] = e
	return nil
}
func (r *fakeTimetableRepo) GetByID(_ context.Context, tenant, id string) (*domain.TimetableEntry, error) {
	e, ok := r.entries[id]
	if !ok || e.TenantID != tenant {
		return nil, domain.ErrNotFound
	}
	return e, nil
}
func (r *fakeTimetableRepo) List(_ context.Context, tenant string, f ports.TimetableFilter) ([]*domain.TimetableEntry, error) {
	var out []*domain.TimetableEntry
	for _, e := range r.entries {
		if e.TenantID != tenant || f.Status != "" && e.Status != f.Status || f.ClassIDs != nil && !idIn(f.ClassIDs, e.ClassID) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}
func (r *fakeTimetableRepo) Update(_ context.Context, _ string, e *domain.TimetableEntry) error {
	r.entries[e.ID] = e
	return nil
}
func (r *fakeTimetableRepo) Delete(_ context.Context, _ string, id string) error {
	delete(r.entries, id)
	return nil
}
func idIn(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

type classScopeStub struct {
	ids []string
	err error
}

func (s classScopeStub) Resolve(context.Context, string, string, string) ([]string, error) {
	return s.ids, s.err
}

func timetableService(t *testing.T, resolver ports.LearnerScopeResolver) (*application.Service, *fakeTimetableRepo, *fakeDB, string, string, string) {
	t.Helper()
	db := newFakeDB()
	year := seedYear(t, db, svcTenantA)
	term, err := domain.NewTerm(svcTenantA, year.ID, "Term 1", "2026-01-01", "2026-04-01")
	if err != nil {
		t.Fatal(err)
	}
	if err := (&fakeTermRepo{db}).Create(context.Background(), svcTenantA, term); err != nil {
		t.Fatalf("create term: %v", err)
	}
	class, err := domain.NewClass(svcTenantA, year.ID, "Form 1", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := (&fakeClassRepo{db}).Create(context.Background(), svcTenantA, class); err != nil {
		t.Fatalf("create class: %v", err)
	}
	subject, err := domain.NewSubject(svcTenantA, "Mathematics", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := (&fakeSubjectRepo{db}).Create(context.Background(), svcTenantA, subject); err != nil {
		t.Fatalf("create subject: %v", err)
	}
	repo := &fakeTimetableRepo{entries: map[string]*domain.TimetableEntry{}}
	g := flags.NewStaticSnapshot()
	g.Set(svcTenantA, application.FeatureAcademicManagement, true)
	g.Set(svcTenantA, application.FeatureTimetable, true)
	svc := application.NewService(&fakeYearRepo{db}, &fakeTermRepo{db}, &fakeClassRepo{db}, &fakeSubjectRepo{db}, application.WithFeatureGate(g), application.WithTimetableRepository(repo), application.WithLearnerScopeResolver(resolver))
	return svc, repo, db, class.ID, term.ID, subject.ID
}

func TestTimetableLearnerScopeAndFailClosed(t *testing.T) {
	// Build once to obtain real class IDs, then configure the resolver on a second service sharing the same repositories.
	base, repo, db, classID, termID, subjectID := timetableService(t, nil)
	_ = base
	g := flags.NewStaticSnapshot()
	g.Set(svcTenantA, application.FeatureAcademicManagement, true)
	g.Set(svcTenantA, application.FeatureTimetable, true)
	svc := application.NewService(&fakeYearRepo{db}, &fakeTermRepo{db}, &fakeClassRepo{db}, &fakeSubjectRepo{db}, application.WithFeatureGate(g), application.WithTimetableRepository(repo), application.WithLearnerScopeResolver(classScopeStub{ids: []string{classID}}))
	entry, err := domain.NewTimetableEntry(svcTenantA, classID, termID, subjectID, nil, 1, "08:00", "09:00", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(context.Background(), svcTenantA, entry); err != nil {
		t.Fatalf("create scoped entry: %v", err)
	}
	other, err := domain.NewTimetableEntry(svcTenantA, "other-class", termID, subjectID, nil, 1, "09:00", "10:00", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(context.Background(), svcTenantA, other); err != nil {
		t.Fatalf("create other entry: %v", err)
	}
	actor := auth.Actor{UserID: "student-1", TenantID: svcTenantA, Role: "student", Permissions: []string{application.PermRead}}
	records, err := svc.ListTimetable(svcCtx(svcTenantA), actor, ports.TimetableFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ID != entry.ID {
		t.Fatalf("expected current class entry only, got %+v", records)
	}
	closed := application.NewService(&fakeYearRepo{db}, &fakeTermRepo{db}, &fakeClassRepo{db}, &fakeSubjectRepo{db}, application.WithFeatureGate(g), application.WithTimetableRepository(repo))
	if _, err := closed.ListTimetable(svcCtx(svcTenantA), actor, ports.TimetableFilter{}); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("expected fail closed, got %v", err)
	}
}

func TestTimetableRejectsInvalidPeriod(t *testing.T) {
	if _, err := domain.NewTimetableEntry(svcTenantA, "class", "term", "subject", nil, 1, "10:00", "09:00", nil); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}
