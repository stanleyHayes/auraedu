package unit

import (
	"context"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

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
	jobs       map[string]*domain.GenerationJob

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
		jobs:       map[string]*domain.GenerationJob{},
	}
}

func (f *fakeRepo) EnqueueReportGeneration(_ context.Context, tenantID, reportCardID string) (*domain.ReportCard, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	card, ok := f.cards[reportCardID]
	if !ok || card.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	if card.Status != string(domain.ReportCardStatusDraft) && card.Status != string(domain.ReportCardStatusPublished) {
		return nil, domain.ErrConflict
	}
	card.SetGenerating()
	f.jobs[reportCardID] = &domain.GenerationJob{ReportCardID: reportCardID, TenantID: tenantID, NextAttemptAt: time.Now().UTC()}
	return card, nil
}

func (f *fakeRepo) ClaimReportGeneration(_ context.Context, lease time.Duration) (*domain.GenerationJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, job := range f.jobs {
		if job.LeaseExpires != nil && job.LeaseExpires.After(time.Now()) {
			continue
		}
		job.Attempts++
		expires := time.Now().Add(lease)
		job.LeaseExpires = &expires
		f.jobs[id] = job
		jobCopy := *job
		return &jobCopy, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeRepo) CompleteReportGeneration(_ context.Context, job *domain.GenerationJob, storagePath string) (*domain.ReportCard, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	card, ok := f.cards[job.ReportCardID]
	if !ok || card.TenantID != job.TenantID || card.Status != string(domain.ReportCardStatusGenerating) {
		return nil, domain.ErrConflict
	}
	card.SetPublished(storagePath)
	delete(f.jobs, job.ReportCardID)
	return card, nil
}

func (f *fakeRepo) RetryReportGeneration(_ context.Context, job *domain.GenerationJob, _ string, maxAttempts int) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	terminal := job.Attempts >= maxAttempts
	if terminal {
		if card := f.cards[job.ReportCardID]; card != nil {
			status := string(domain.ReportCardStatusDraft)
			if _, err := card.ApplyUpdate(nil, nil, nil, &status); err != nil {
				return false, err
			}
		}
		delete(f.jobs, job.ReportCardID)
		return true, nil
	}
	if queued := f.jobs[job.ReportCardID]; queued != nil {
		queued.LeaseExpires = nil
	}
	return false, nil
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

func (f *fakeRepo) ListReportCards(_ context.Context, tenantID string, filter ports.ReportCardListFilter) ([]*domain.ReportCard, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.ReportCard
	for _, c := range f.cards {
		if c.TenantID != tenantID || (filter.Status != "" && c.Status != filter.Status) {
			continue
		}
		if filter.StudentID != "" && c.StudentID != filter.StudentID {
			continue
		}
		if filter.StudentIDs != nil && !containsString(filter.StudentIDs, c.StudentID) {
			continue
		}
		out = append(out, c)
	}
	return out, "", nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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

func (f *fakeRepo) ListTranscriptReportCards(_ context.Context, tenantID, studentID string) ([]*domain.ReportCard, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.ReportCard
	for _, card := range f.cards {
		if card.TenantID == tenantID && card.StudentID == studentID && card.DeletedAt == nil &&
			(card.Status == string(domain.ReportCardStatusPublished) || card.Status == string(domain.ReportCardStatusArchived)) {
			out = append(out, card)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
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

type fakeReportStorage struct {
	objects map[string]string
	fail    error
}

func newFakeReportStorage() *fakeReportStorage {
	return &fakeReportStorage{objects: map[string]string{}}
}

func (f *fakeReportStorage) Save(_ context.Context, tenantID, objectKey string, content []byte) (string, error) {
	if f.fail != nil {
		return "", f.fail
	}
	path := tenantID + "/" + objectKey
	f.objects[path] = string(content)
	return path, nil
}

func (f *fakeReportStorage) Open(_ context.Context, tenantID, storagePath string) (io.ReadCloser, error) {
	if f.fail != nil {
		return nil, f.fail
	}
	if !strings.HasPrefix(storagePath, tenantID+"/") {
		return nil, domain.ErrForbidden
	}
	content, ok := f.objects[storagePath]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return io.NopCloser(strings.NewReader(content)), nil
}
