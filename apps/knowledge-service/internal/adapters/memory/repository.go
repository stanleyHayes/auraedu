// Package memory provides an in-memory knowledge repository for tests.
package memory

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/auraedu/knowledge-service/internal/domain"
)

type Repository struct {
	mu      sync.RWMutex
	sources map[string]domain.Source
}

func New() *Repository { return &Repository{sources: map[string]domain.Source{}} }

func key(tenantID, id string) string { return tenantID + ":" + id }

func (r *Repository) Create(_ context.Context, source domain.Source) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(source.TenantID, source.ID)
	if _, exists := r.sources[k]; exists {
		return domain.ErrConflict
	}
	r.sources[k] = source
	return nil
}

func (r *Repository) Get(_ context.Context, tenantID, id string) (domain.Source, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	source, ok := r.sources[key(tenantID, id)]
	if !ok {
		return domain.Source{}, domain.ErrNotFound
	}
	return source, nil
}

func (r *Repository) List(_ context.Context, tenantID string, status domain.Status, limit int) ([]domain.Source, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]domain.Source, 0)
	for _, source := range r.sources {
		if source.TenantID == tenantID && (status == "" || source.Status == status) {
			items = append(items, source)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *Repository) Approve(_ context.Context, tenantID, id, reviewer, note string, now time.Time) (domain.Source, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(tenantID, id)
	source, ok := r.sources[k]
	if !ok {
		return domain.Source{}, domain.ErrNotFound
	}
	if source.Status != domain.StatusDraft {
		return domain.Source{}, domain.ErrConflict
	}
	source.Status, source.ApprovedBy, source.ApprovedAt, source.ReviewNote, source.UpdatedAt = domain.StatusApproved, &reviewer, &now, &note, now
	r.sources[k] = source
	return source, nil
}

func (r *Repository) Retire(_ context.Context, tenantID, id string, now time.Time) (domain.Source, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(tenantID, id)
	source, ok := r.sources[k]
	if !ok {
		return domain.Source{}, domain.ErrNotFound
	}
	if source.Status != domain.StatusApproved {
		return domain.Source{}, domain.ErrConflict
	}
	source.Status, source.UpdatedAt = domain.StatusRetired, now
	r.sources[k] = source
	return source, nil
}

func (r *Repository) Search(_ context.Context, tenantID, query, locale string, limit int, asOf time.Time) ([]domain.SearchResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	terms := tokens(query)
	results := make([]domain.SearchResult, 0)
	for _, source := range r.sources {
		if source.TenantID != tenantID || !source.IsRetrievable(asOf) || !domain.SameLanguage(source.Locale, locale) {
			continue
		}
		haystack := tokens(source.Title + " " + source.Content + " " + optional(source.Programme))
		matches := 0
		for term := range terms {
			if haystack[term] {
				matches++
			}
		}
		if matches == 0 {
			continue
		}
		score := math.Min(1, float64(matches)/float64(len(terms)))
		results = append(results, domain.SearchResult{SourceID: source.ID, Title: source.Title,
			Passage: passage(source.Content, terms), SourceType: source.SourceType, Locale: source.Locale, Version: source.Version,
			Score: score, EffectiveAt: source.EffectiveAt, ExpiresAt: source.ExpiresAt})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].EffectiveAt.After(results[j].EffectiveAt)
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func tokens(value string) map[string]bool {
	fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	result := make(map[string]bool, len(fields))
	for _, field := range fields {
		if len(field) > 1 {
			result[field] = true
		}
	}
	return result
}

func passage(content string, terms map[string]bool) string {
	const maximum = 800
	if len(content) <= maximum {
		return content
	}
	lower := strings.ToLower(content)
	start := 0
	for term := range terms {
		if index := strings.Index(lower, term); index >= 0 {
			start = index - 160
			if start < 0 {
				start = 0
			}
			break
		}
	}
	end := start + maximum
	if end > len(content) {
		end = len(content)
	}
	return strings.TrimSpace(content[start:end])
}

func optional(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
