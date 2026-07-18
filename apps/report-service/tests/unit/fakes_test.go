package unit

import (
	"context"
	"sort"
	"sync"

	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
)

// fakeRepo is an in-memory ports.Repository for application-layer unit tests.
// It enforces tenant scoping the same way the Postgres adapter does: every
// lookup filters by the tenantID argument.
type fakeRepo struct {
	mu sync.Mutex

	templates  map[string]*domain.ReportTemplate
	cards      map[string]*domain.ReportCard
	scores     map[string]*domain.ScoreEntry      // key: cardID|sourceKey
	attendance map[string]*domain.AttendanceEntry // key: cardID|date

	// failNextCreateWithConflict simulates a concurrent auto-create losing the
	// unique-index race exactly once.
	failNextCreateWithConflict bool
	// hideNextFindDraft makes the next FindDraftReportCard report not-found,
	// simulating the read that races a concurrent create.
	hideNextFindDraft bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		templates:  map[string]*domain.ReportTemplate{},
		cards:      map[string]*domain.ReportCard{},
		scores:     map[string]*domain.ScoreEntry{},
		attendance: map[string]*domain.AttendanceEntry{},
	}
}

func (f *fakeRepo) CreateReportTemplate(_ context.Context, tenantID string, t *domain.ReportTemplate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t.TenantID != tenantID {
		return domain.ErrForbidden
	}
	f.templates[t.ID] = t
	return nil
}

func (f *fakeRepo) GetReportTemplateByID(_ context.Context, tenantID, id string) (*domain.ReportTemplate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.templates[id]
	if !ok || t.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func (f *fakeRepo) ListReportTemplates(_ context.Context, tenantID string, _ ports.ReportTemplateListFilter) ([]*domain.ReportTemplate, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.ReportTemplate
	for _, t := range f.templates {
		if t.TenantID == tenantID {
			out = append(out, t)
		}
	}
	return out, "", nil
}

func (f *fakeRepo) UpdateReportTemplate(_ context.Context, tenantID string, t *domain.ReportTemplate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.templates[t.ID]; !ok || f.templates[t.ID].TenantID != tenantID {
		return domain.ErrNotFound
	}
	f.templates[t.ID] = t
	return nil
}

func (f *fakeRepo) DeleteReportTemplate(_ context.Context, tenantID, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.templates[id]; !ok || f.templates[id].TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(f.templates, id)
	return nil
}

func (f *fakeRepo) CreateReportCard(_ context.Context, tenantID string, c *domain.ReportCard) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNextCreateWithConflict {
		f.failNextCreateWithConflict = false
		return domain.ErrConflict
	}
	if c.TenantID != tenantID {
		return domain.ErrForbidden
	}
	f.cards[c.ID] = c
	return nil
}

func (f *fakeRepo) GetReportCardByID(_ context.Context, tenantID, id string) (*domain.ReportCard, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.cards[id]
	if !ok || c.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeRepo) ListReportCards(_ context.Context, tenantID string, _ ports.ReportCardListFilter) ([]*domain.ReportCard, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.ReportCard
	for _, c := range f.cards {
		if c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	return out, "", nil
}

func (f *fakeRepo) UpdateReportCard(_ context.Context, tenantID string, c *domain.ReportCard) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.cards[c.ID]; !ok || f.cards[c.ID].TenantID != tenantID {
		return domain.ErrNotFound
	}
	f.cards[c.ID] = c
	return nil
}

func (f *fakeRepo) DeleteReportCard(_ context.Context, tenantID, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.cards[id]; !ok || f.cards[id].TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(f.cards, id)
	return nil
}

// FindDraftReportCard mirrors the Postgres semantics: with a non-empty termID,
// NULL-term drafts also match and exact term matches win; with an empty termID
// every draft matches and the most recently created wins.
func (f *fakeRepo) FindDraftReportCard(_ context.Context, tenantID, studentID, termID string) (*domain.ReportCard, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.hideNextFindDraft {
		f.hideNextFindDraft = false
		return nil, domain.ErrNotFound
	}
	var matches []*domain.ReportCard
	for _, c := range f.cards {
		if c.TenantID != tenantID || c.StudentID != studentID {
			continue
		}
		if c.Status != string(domain.ReportCardStatusDraft) || c.DeletedAt != nil {
			continue
		}
		if termID != "" && c.TermID != "" && c.TermID != termID {
			continue
		}
		matches = append(matches, c)
	}
	if len(matches) == 0 {
		return nil, domain.ErrNotFound
	}
	sort.Slice(matches, func(i, j int) bool {
		if termID != "" {
			iExact, jExact := matches[i].TermID == termID, matches[j].TermID == termID
			if iExact != jExact {
				return iExact
			}
		}
		if !matches[i].CreatedAt.Equal(matches[j].CreatedAt) {
			return matches[i].CreatedAt.After(matches[j].CreatedAt)
		}
		return matches[i].ID > matches[j].ID
	})
	return matches[0], nil
}

func (f *fakeRepo) UpsertScoreEntry(_ context.Context, tenantID string, e *domain.ScoreEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if e.TenantID != tenantID {
		return domain.ErrForbidden
	}
	key := e.ReportCardID + "|" + e.SourceKey
	if existing, ok := f.scores[key]; ok {
		existing.SubjectID = e.SubjectID
		existing.Score = e.Score
		existing.MaxScore = e.MaxScore
		existing.LastEventID = e.LastEventID
		existing.UpdatedAt = e.UpdatedAt
		return nil
	}
	f.scores[key] = e
	return nil
}

func (f *fakeRepo) UpsertAttendanceEntry(_ context.Context, tenantID string, e *domain.AttendanceEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if e.TenantID != tenantID {
		return domain.ErrForbidden
	}
	key := e.ReportCardID + "|" + e.Date.Format("2006-01-02")
	if existing, ok := f.attendance[key]; ok {
		existing.Status = e.Status
		existing.LastEventID = e.LastEventID
		existing.UpdatedAt = e.UpdatedAt
		return nil
	}
	f.attendance[key] = e
	return nil
}

func (f *fakeRepo) ListScoreEntries(_ context.Context, tenantID, reportCardID string) ([]*domain.ScoreEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.ScoreEntry
	for _, e := range f.scores {
		if e.TenantID == tenantID && e.ReportCardID == reportCardID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListAttendanceEntries(_ context.Context, tenantID, reportCardID string) ([]*domain.AttendanceEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.AttendanceEntry
	for _, e := range f.attendance {
		if e.TenantID == tenantID && e.ReportCardID == reportCardID {
			out = append(out, e)
		}
	}
	return out, nil
}

// --- Test doubles for the other ports. ---

type fakePDFGenerator struct {
	lastDoc *domain.ReportCardDocument
}

func (f *fakePDFGenerator) GenerateReportCard(_ context.Context, doc *domain.ReportCardDocument) ([]byte, error) {
	f.lastDoc = doc
	return []byte("%PDF-1.7 fake"), nil
}

type publishedEvent struct {
	eventType string
	card      *domain.ReportCard
	template  *domain.ReportTemplate
}

type fakePublisher struct {
	events []publishedEvent
}

func (f *fakePublisher) PublishReportTemplate(_ context.Context, eventType string, t *domain.ReportTemplate, _ map[string]any) error {
	f.events = append(f.events, publishedEvent{eventType: eventType, template: t})
	return nil
}

func (f *fakePublisher) PublishReportCard(_ context.Context, eventType string, c *domain.ReportCard, _ map[string]any) error {
	f.events = append(f.events, publishedEvent{eventType: eventType, card: c})
	return nil
}
