// Package memory provides the development Admissions repository.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/auraedu/admissions-service/internal/domain"
)

type Repository struct {
	mu         sync.RWMutex
	items      map[string]domain.Application
	programmes map[string]domain.Programme
	intakes    map[string]domain.Intake
}

func New() *Repository {
	return &Repository{items: map[string]domain.Application{}, programmes: map[string]domain.Programme{}, intakes: map[string]domain.Intake{}}
}
func key(t, id string) string { return t + ":" + id }
func (r *Repository) Create(_ context.Context, a domain.Application) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(a.TenantID, a.ID)
	if _, ok := r.items[k]; ok {
		return domain.ErrConflict
	}
	r.items[k] = a
	return nil
}
func (r *Repository) Get(_ context.Context, t, id string) (domain.Application, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.items[key(t, id)]
	if !ok {
		return domain.Application{}, domain.ErrNotFound
	}
	return a, nil
}
func (r *Repository) List(_ context.Context, t, applicant string, status domain.Status, limit int) ([]domain.Application, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []domain.Application{}
	for _, a := range r.items {
		if a.TenantID == t && (applicant == "" || a.ApplicantUserID == applicant) && (status == "" || a.Status == status) {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (r *Repository) Update(_ context.Context, a domain.Application, expected domain.Status) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(a.TenantID, a.ID)
	old, ok := r.items[k]
	if !ok {
		return domain.ErrNotFound
	}
	if old.Status != expected {
		return domain.ErrConflict
	}
	r.items[k] = a
	return nil
}

func (r *Repository) CreateProgramme(_ context.Context, programme domain.Programme) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.programmes {
		if item.TenantID == programme.TenantID && (item.Code == programme.Code || item.Slug == programme.Slug) {
			return domain.ErrConflict
		}
	}
	r.programmes[key(programme.TenantID, programme.ID)] = programme
	return nil
}

func (r *Repository) GetProgramme(_ context.Context, tenantID, id string) (domain.Programme, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	programme, ok := r.programmes[key(tenantID, id)]
	if !ok {
		return domain.Programme{}, domain.ErrNotFound
	}
	programme.Intakes = r.programmeIntakes(tenantID, id, false, time.Time{})
	return programme, nil
}

func (r *Repository) ListProgrammes(_ context.Context, tenantID string, public bool, now time.Time, limit int) ([]domain.Programme, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := []domain.Programme{}
	for _, programme := range r.programmes {
		if programme.TenantID != tenantID || (public && programme.Status != domain.ProgrammePublished) {
			continue
		}
		programme.Intakes = r.programmeIntakes(tenantID, programme.ID, public, now)
		if public && len(programme.Intakes) == 0 {
			continue
		}
		items = append(items, programme)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *Repository) programmeIntakes(tenantID, programmeID string, public bool, now time.Time) []domain.Intake {
	items := []domain.Intake{}
	for _, intake := range r.intakes {
		if intake.TenantID == tenantID && intake.ProgrammeID == programmeID && (!public || intake.IsAvailable(now)) {
			items = append(items, intake)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].StartsAt.Equal(items[j].StartsAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].StartsAt.Before(items[j].StartsAt)
	})
	return items
}

func (r *Repository) UpdateProgramme(_ context.Context, programme domain.Programme, expectedVersion int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(programme.TenantID, programme.ID)
	current, ok := r.programmes[k]
	if !ok {
		return domain.ErrNotFound
	}
	if current.Version != expectedVersion {
		return domain.ErrConflict
	}
	for _, item := range r.programmes {
		if item.TenantID == programme.TenantID && item.ID != programme.ID && (item.Code == programme.Code || item.Slug == programme.Slug) {
			return domain.ErrConflict
		}
	}
	programme.Intakes = nil
	r.programmes[k] = programme
	return nil
}

func (r *Repository) CreateIntake(_ context.Context, intake domain.Intake) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.programmes[key(intake.TenantID, intake.ProgrammeID)]; !ok {
		return domain.ErrNotFound
	}
	for _, item := range r.intakes {
		if item.TenantID == intake.TenantID && item.ProgrammeID == intake.ProgrammeID && item.Name == intake.Name && item.StartsAt.Equal(intake.StartsAt) {
			return domain.ErrConflict
		}
	}
	r.intakes[key(intake.TenantID, intake.ID)] = intake
	return nil
}

func (r *Repository) GetIntake(_ context.Context, tenantID, id string) (domain.Intake, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	intake, ok := r.intakes[key(tenantID, id)]
	if !ok {
		return domain.Intake{}, domain.ErrNotFound
	}
	return intake, nil
}

func (r *Repository) UpdateIntake(_ context.Context, intake domain.Intake, expectedVersion int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(intake.TenantID, intake.ID)
	current, ok := r.intakes[k]
	if !ok {
		return domain.ErrNotFound
	}
	if current.Version != expectedVersion {
		return domain.ErrConflict
	}
	for _, item := range r.intakes {
		sameIntake := item.TenantID == intake.TenantID &&
			item.ProgrammeID == intake.ProgrammeID && item.ID != intake.ID &&
			item.Name == intake.Name && item.StartsAt.Equal(intake.StartsAt)
		if sameIntake {
			return domain.ErrConflict
		}
	}
	r.intakes[k] = intake
	return nil
}

func (r *Repository) ResolveAvailableIntake(_ context.Context, tenantID, programmeID, intakeID string, now time.Time) (domain.Programme, domain.Intake, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	programme, ok := r.programmes[key(tenantID, programmeID)]
	if !ok || programme.Status != domain.ProgrammePublished {
		return domain.Programme{}, domain.Intake{}, domain.ErrNotFound
	}
	intake, ok := r.intakes[key(tenantID, intakeID)]
	if !ok || intake.ProgrammeID != programmeID || !intake.IsAvailable(now) {
		return domain.Programme{}, domain.Intake{}, domain.ErrNotFound
	}
	programme.Intakes = []domain.Intake{intake}
	return programme, intake, nil
}

func (r *Repository) CreateForAvailableIntake(_ context.Context, application domain.Application, now time.Time, _ string, _ map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	programme, ok := r.programmes[key(application.TenantID, application.ProgrammeID)]
	if !ok || programme.Status != domain.ProgrammePublished {
		return domain.ErrNotFound
	}
	intake, ok := r.intakes[key(application.TenantID, application.IntakeID)]
	if !ok || intake.ProgrammeID != application.ProgrammeID || !intake.IsAvailable(now) {
		return domain.ErrNotFound
	}
	k := key(application.TenantID, application.ID)
	if _, exists := r.items[k]; exists {
		return domain.ErrConflict
	}
	for _, item := range r.items {
		sameApplicantIntake := item.TenantID == application.TenantID &&
			item.ApplicantUserID == application.ApplicantUserID &&
			item.ProgrammeID == application.ProgrammeID && item.IntakeID == application.IntakeID
		if sameApplicantIntake {
			return domain.ErrConflict
		}
	}
	r.items[k] = application
	return nil
}
