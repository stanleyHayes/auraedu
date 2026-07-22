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
	"github.com/auraedu/platform/flags"
)

func validRanges() []domain.GradeRange {
	return []domain.GradeRange{
		{Min: 80, Max: 100, Grade: "A", Remark: "Excellent"},
		{Min: 60, Max: 79.99, Grade: "B", Remark: "Good"},
		{Min: 0, Max: 59.99, Grade: "C", Remark: "Developing"},
	}
}

func TestGradingScaleRejectsInvalidAndOverlappingRanges(t *testing.T) {
	if _, err := domain.NewGradingScale(svcTenantA, "", validRanges()); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("empty name: %v", err)
	}
	if _, err := domain.NewGradingScale(svcTenantA, "Standard", nil); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("empty ranges: %v", err)
	}
	overlap := []domain.GradeRange{{Min: 0, Max: 60, Grade: "B"}, {Min: 60, Max: 100, Grade: "A"}}
	if _, err := domain.NewGradingScale(svcTenantA, "Standard", overlap); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("overlap: %v", err)
	}
	if _, err := domain.NewGradingScale(svcTenantA, "Standard", []domain.GradeRange{{Min: -1, Max: 101, Grade: "A"}}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("out of range: %v", err)
	}
}

type fakeGradingRepo struct {
	mu    sync.Mutex
	items map[string]*domain.GradingScale
}

var _ ports.GradingScaleRepository = (*fakeGradingRepo)(nil)

func newFakeGradingRepo() *fakeGradingRepo {
	return &fakeGradingRepo{items: make(map[string]*domain.GradingScale)}
}

func (r *fakeGradingRepo) Create(_ context.Context, tenantID string, scale *domain.GradingScale) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	scale.TenantID = tenantID
	r.items[scale.ID] = scale
	return nil
}

func (r *fakeGradingRepo) GetByID(_ context.Context, tenantID, id string) (*domain.GradingScale, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	scale, ok := r.items[id]
	if !ok || scale.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return scale, nil
}

func (r *fakeGradingRepo) List(_ context.Context, tenantID string, limit int, _ string) ([]*domain.GradingScale, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]*domain.GradingScale, 0)
	for _, scale := range r.items {
		if scale.TenantID == tenantID {
			items = append(items, scale)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if len(items) > limit {
		return items[:limit], items[limit-1].ID, nil
	}
	return items, "", nil
}

func (r *fakeGradingRepo) Update(_ context.Context, tenantID string, scale *domain.GradingScale) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.items[scale.ID]
	if !ok || current.TenantID != tenantID {
		return domain.ErrNotFound
	}
	r.items[scale.ID] = scale
	return nil
}

func (r *fakeGradingRepo) Delete(_ context.Context, tenantID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.items[id]
	if !ok || current.TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(r.items, id)
	return nil
}

func TestServiceGradingScaleLifecycleIsTenantScoped(t *testing.T) {
	db := newFakeDB()
	repo := newFakeGradingRepo()
	gates := flags.NewStaticSnapshot()
	gates.Set(svcTenantA, application.FeatureAcademicManagement, true)
	gates.Set(svcTenantB, application.FeatureAcademicManagement, true)
	svc := application.NewService(
		&fakeYearRepo{db}, &fakeTermRepo{db}, &fakeClassRepo{db}, &fakeSubjectRepo{db},
		application.WithFeatureGate(gates),
		application.WithGradingScaleRepository(repo),
	)
	managerA := svcActor(svcTenantA, application.PermRead, application.PermManage)
	scale, err := svc.CreateGradingScale(svcCtx(svcTenantA), managerA, application.CreateGradingScaleRequest{Name: "SHS standard", Ranges: validRanges()})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if scale.Ranges[0].Min != 0 || scale.Ranges[2].Max != 100 {
		t.Fatalf("ranges were not normalized: %+v", scale.Ranges)
	}
	items, _, err := svc.ListGradingScales(svcCtx(svcTenantA), managerA, 25, "")
	if err != nil || len(items) != 1 {
		t.Fatalf("list: len=%d err=%v", len(items), err)
	}
	readerB := svcActor(svcTenantB, application.PermRead)
	if _, err := svc.GetGradingScale(svcCtx(svcTenantB), readerB, scale.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant get: %v", err)
	}
	name := "SHS revised"
	updated, err := svc.UpdateGradingScale(svcCtx(svcTenantA), managerA, scale.ID, application.UpdateGradingScaleRequest{Name: &name})
	if err != nil || updated.Name != name {
		t.Fatalf("update: record=%+v err=%v", updated, err)
	}
	if err := svc.DeleteGradingScale(svcCtx(svcTenantA), managerA, scale.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.GetGradingScale(svcCtx(svcTenantA), managerA, scale.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted get: %v", err)
	}
}
