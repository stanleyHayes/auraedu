package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	svchttp "github.com/auraedu/student-service/internal/adapters/http"
	"github.com/auraedu/student-service/internal/application"
	"github.com/auraedu/student-service/internal/domain"
	"github.com/auraedu/student-service/internal/ports"
)

const tenantA = "11111111-1111-1111-1111-111111111111"

// fakeRepo is an in-memory ports.Repository for fast handler tests.
type fakeRepo struct {
	mu   sync.Mutex
	data map[string]*domain.Student
}

func newFakeRepo() *fakeRepo { return &fakeRepo{data: make(map[string]*domain.Student)} }

func (r *fakeRepo) Create(_ context.Context, tenantID string, s *domain.Student) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s.TenantID = tenantID
	r.data[s.ID] = s
	return nil
}

func (r *fakeRepo) GetByID(_ context.Context, tenantID, id string) (*domain.Student, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.data[id]
	if !ok || s.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return s, nil
}

func (r *fakeRepo) List(_ context.Context, tenantID string, limit int, cursor string) ([]*domain.Student, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Student
	for _, s := range r.data {
		if s.TenantID == tenantID {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	start := 0
	if cursor != "" {
		for i, s := range out {
			if s.ID == cursor {
				start = i + 1
				break
			}
		}
	}
	end := start + limit
	if end > len(out) {
		end = len(out)
	}
	page := out[start:end]
	var next string
	if end < len(out) {
		next = out[end-1].ID
	}
	return page, next, nil
}

func (r *fakeRepo) Update(_ context.Context, tenantID string, s *domain.Student) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.data[s.ID]
	if !ok || existing.TenantID != tenantID {
		return domain.ErrNotFound
	}
	r.data[s.ID] = s
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, tenantID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.data[id]
	if !ok || existing.TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(r.data, id)
	return nil
}

var _ ports.Repository = (*fakeRepo)(nil)

func newTestHandler() (*svchttp.Handler, *fakeRepo) {
	repo := newFakeRepo()
	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureStudentManagement, true)
	svc := application.NewService(repo, application.WithFeatureGate(gates))
	return svchttp.NewHandler(svc), repo
}

func request(t *testing.T, method, path string, body any, perms ...string) *http.Request {
	t.Helper()
	var bodyReader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(tenancy.HeaderTenantID, tenantA)
	req.Header.Set(auth.HeaderUserID, "user-1")
	req.Header.Set(auth.HeaderTenant, tenantA)
	req.Header.Set(auth.HeaderRole, "school_admin")
	if len(perms) > 0 {
		req.Header.Set(auth.HeaderPermissions, strings.Join(perms, ","))
	}
	return req
}

func TestHandler_CreateStudent(t *testing.T) {
	h, _ := newTestHandler()
	req := request(t, http.MethodPost, "/api/v1/students", map[string]any{
		"first_name": "Kwame",
		"last_name":  "Nkrumah",
	}, application.PermCreate)
	rr := httptest.NewRecorder()
	h.Register(http.NewServeMux()) // not used directly; call handler via route
	// Use the handler method directly to avoid route setup noise.
	h.Register(http.NewServeMux())
	// Serve via a fresh mux registered with the handler.
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["first_name"] != "Kwame" || got["last_name"] != "Nkrumah" {
		t.Fatalf("unexpected student: %v", got)
	}
}

func TestHandler_CreateStudent_Forbidden(t *testing.T) {
	h, _ := newTestHandler()
	req := request(t, http.MethodPost, "/api/v1/students", map[string]any{
		"first_name": "X",
		"last_name":  "Y",
	})
	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandler_ListStudents(t *testing.T) {
	h, repo := newTestHandler()
	ctx := context.Background()
	s1, _ := domain.NewStudent(tenantA, "A", "One")
	s2, _ := domain.NewStudent(tenantA, "B", "Two")
	_ = repo.Create(ctx, tenantA, s1)
	_ = repo.Create(ctx, tenantA, s2)

	req := request(t, http.MethodGet, "/api/v1/students", nil, application.PermRead)
	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := got["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("expected 2 students, got %v", got["data"])
	}
}

func TestHandler_GetStudent_NotFound(t *testing.T) {
	h, _ := newTestHandler()
	req := request(t, http.MethodGet, "/api/v1/students/nonexistent", nil, application.PermRead)
	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandler_FeatureDisabled(t *testing.T) {
	repo := newFakeRepo()
	gates := flags.NewStaticSnapshot() // all disabled
	svc := application.NewService(repo, application.WithFeatureGate(gates))
	h := svchttp.NewHandler(svc)

	req := request(t, http.MethodPost, "/api/v1/students", map[string]any{
		"first_name": "X",
		"last_name":  "Y",
	}, application.PermCreate)
	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

// Ensure fakeRepo error handling stays aligned with domain errors.
func TestFakeRepo_Isolation(t *testing.T) {
	repo := newFakeRepo()
	ctx := context.Background()
	s, _ := domain.NewStudent(tenantA, "Tenant", "A")
	if err := repo.Create(ctx, tenantA, s); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.GetByID(ctx, "other-tenant", s.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for other tenant, got %v", err)
	}
}
