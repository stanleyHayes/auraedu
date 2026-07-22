// Package memory provides an in-memory content repository for tests.
//
//nolint:lll // Compact test repository operations mirror the production port in one place.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/auraedu/content-service/internal/domain"
	"github.com/auraedu/content-service/internal/ports"
)

type replay struct {
	Draft       domain.Draft
	RequestHash string
}

type Repository struct {
	mu       sync.RWMutex
	profiles map[string]domain.BrandProfile
	drafts   map[string]map[string]domain.Draft
	versions map[string]map[string][]domain.Version
	replays  map[string]map[string]replay
}

func NewRepository() *Repository {
	return &Repository{profiles: map[string]domain.BrandProfile{}, drafts: map[string]map[string]domain.Draft{}, versions: map[string]map[string][]domain.Version{}, replays: map[string]map[string]replay{}}
}

func (r *Repository) GetBrandProfile(_ context.Context, tenantID string) (domain.BrandProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	profile, ok := r.profiles[tenantID]
	if !ok {
		return domain.BrandProfile{}, domain.ErrNotFound
	}
	return profile, nil
}

func (r *Repository) UpsertBrandProfile(_ context.Context, profile domain.BrandProfile, expected int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, exists := r.profiles[profile.TenantID]
	if (exists && current.Version != expected) || (!exists && expected != 0) {
		return domain.ErrConflict
	}
	r.profiles[profile.TenantID] = profile
	return nil
}

func (r *Repository) FindReplay(_ context.Context, tenantID, keyHash, _ string) (domain.Draft, string, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.replays[tenantID][keyHash]
	return item.Draft, item.RequestHash, ok, nil
}

func (r *Repository) CreateDraftWithEvent(_ context.Context, draft domain.Draft, version domain.Version, keyHash, requestHash string, _ map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.drafts[draft.TenantID] == nil {
		r.drafts[draft.TenantID], r.versions[draft.TenantID], r.replays[draft.TenantID] = map[string]domain.Draft{}, map[string][]domain.Version{}, map[string]replay{}
	}
	if existing, ok := r.replays[draft.TenantID][keyHash]; ok {
		if existing.RequestHash != requestHash {
			return domain.ErrConflict
		}
		return nil
	}
	r.drafts[draft.TenantID][draft.ID] = draft
	r.versions[draft.TenantID][draft.ID] = []domain.Version{version}
	r.replays[draft.TenantID][keyHash] = replay{Draft: draft, RequestHash: requestHash}
	return nil
}

func (r *Repository) GetDraft(_ context.Context, tenantID, id string) (domain.Draft, []domain.Version, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	draft, ok := r.drafts[tenantID][id]
	if !ok {
		return domain.Draft{}, nil, domain.ErrNotFound
	}
	versions := append([]domain.Version(nil), r.versions[tenantID][id]...)
	return draft, versions, nil
}

func (r *Repository) ListDrafts(_ context.Context, tenantID string, filter ports.ListFilter) ([]domain.Draft, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := []domain.Draft{}
	for _, draft := range r.drafts[tenantID] {
		if filter.Status != "" && string(draft.Status) != filter.Status || filter.ContentType != "" && draft.ContentType != filter.ContentType || filter.CampaignID != "" && (draft.CampaignID == nil || *draft.CampaignID != filter.CampaignID) {
			continue
		}
		items = append(items, draft)
		if len(items) == filter.Limit {
			break
		}
	}
	return items, nil
}

func (r *Repository) UpdateDraftWithVersionAndEvent(_ context.Context, draft domain.Draft, expected int, expectedUpdatedAt time.Time, version *domain.Version, _ string, _ map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.drafts[draft.TenantID][draft.ID]
	if !ok {
		return domain.ErrNotFound
	}
	if current.Version != expected || !current.UpdatedAt.Equal(expectedUpdatedAt) {
		return domain.ErrConflict
	}
	r.drafts[draft.TenantID][draft.ID] = draft
	if version != nil {
		r.versions[draft.TenantID][draft.ID] = append(r.versions[draft.TenantID][draft.ID], *version)
	}
	return nil
}
