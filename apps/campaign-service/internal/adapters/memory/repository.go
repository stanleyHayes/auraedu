// Package memory provides an in-memory Campaign repository.
package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/auraedu/campaign-service/internal/domain"
)

type Repository struct {
	mu        sync.RWMutex
	campaigns map[string]domain.Campaign
}

func New() *Repository               { return &Repository{campaigns: map[string]domain.Campaign{}} }
func key(tenantID, id string) string { return tenantID + ":" + id }
func (r *Repository) Create(_ context.Context, c domain.Campaign) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(c.TenantID, c.ID)
	if _, ok := r.campaigns[k]; ok {
		return domain.ErrConflict
	}
	r.campaigns[k] = c
	return nil
}
func (r *Repository) Get(_ context.Context, tenantID, id string) (domain.Campaign, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.campaigns[key(tenantID, id)]
	if !ok {
		return domain.Campaign{}, domain.ErrNotFound
	}
	return c, nil
}
func (r *Repository) List(_ context.Context, tenantID string, status domain.Status, limit int) ([]domain.Campaign, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := []domain.Campaign{}
	for _, c := range r.campaigns {
		if c.TenantID == tenantID && (status == "" || c.Status == status) {
			items = append(items, c)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}
func (r *Repository) Update(_ context.Context, c domain.Campaign, expected domain.Status) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(c.TenantID, c.ID)
	current, ok := r.campaigns[k]
	if !ok {
		return domain.ErrNotFound
	}
	if current.Status != expected {
		return domain.ErrConflict
	}
	r.campaigns[k] = c
	return nil
}
