package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
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
	mu          sync.Mutex
	students    map[string]*domain.Student
	guardians   map[string]*domain.Guardian
	links       map[string]*domain.StudentGuardian
	enrollments map[string]*domain.Enrollment
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		students:    make(map[string]*domain.Student),
		guardians:   make(map[string]*domain.Guardian),
		links:       make(map[string]*domain.StudentGuardian),
		enrollments: make(map[string]*domain.Enrollment),
	}
}

func mustNewStudent(t *testing.T, firstName, lastName string) *domain.Student {
	t.Helper()
	student, err := domain.NewStudent(tenantA, firstName, lastName)
	if err != nil {
		t.Fatal(err)
	}
	return student
}

func mustStoreStudent(t *testing.T, repo *fakeRepo, student *domain.Student) {
	t.Helper()
	if err := repo.Create(context.Background(), tenantA, student); err != nil {
		t.Fatal(err)
	}
}

func (r *fakeRepo) Create(_ context.Context, tenantID string, s *domain.Student) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s.TenantID = tenantID
	r.students[s.ID] = s
	if s.ClassID != nil && s.AcademicYearID != nil {
		enrollment, err := domain.NewEnrollment(tenantID, s.ID, *s.ClassID, *s.AcademicYearID, s.CreatedAt)
		if err != nil {
			return err
		}
		r.enrollments[enrollment.ID] = enrollment
	}
	return nil
}

func (r *fakeRepo) CreateEnrollment(_ context.Context, tenantID string, enrollment *domain.Enrollment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	student, ok := r.students[enrollment.StudentID]
	if !ok || student.TenantID != tenantID {
		return domain.ErrNotFound
	}
	for _, existing := range r.enrollments {
		if existing.TenantID == tenantID && existing.StudentID == enrollment.StudentID && existing.AcademicYearID == enrollment.AcademicYearID {
			return domain.ErrConflict
		}
	}
	r.enrollments[enrollment.ID] = enrollment
	student.ClassID = &enrollment.ClassID
	student.AcademicYearID = &enrollment.AcademicYearID
	return nil
}

func (r *fakeRepo) ListEnrollments(_ context.Context, tenantID, studentID string, limit int, cursor string) ([]*domain.Enrollment, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var all []*domain.Enrollment
	for _, enrollment := range r.enrollments {
		if enrollment.TenantID == tenantID && enrollment.StudentID == studentID {
			all = append(all, enrollment)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].EnrolledAt.Before(all[j].EnrolledAt) })
	start := 0
	if cursor != "" {
		for i, enrollment := range all {
			if enrollment.ID == cursor {
				start = i + 1
				break
			}
		}
	}
	end := start + limit
	if end > len(all) {
		end = len(all)
	}
	next := ""
	if end < len(all) && end > start {
		next = all[end-1].ID
	}
	return all[start:end], next, nil
}

func (r *fakeRepo) GetByID(_ context.Context, tenantID, id string) (*domain.Student, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.students[id]
	if !ok || s.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return s, nil
}

func (r *fakeRepo) GetStudentByUserID(_ context.Context, tenantID, userID string) (*domain.Student, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.students {
		if s.TenantID == tenantID && s.UserID != nil && *s.UserID == userID {
			return s, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *fakeRepo) GetGuardianByUserID(_ context.Context, tenantID, userID string) (*domain.Guardian, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, g := range r.guardians {
		if g.TenantID == tenantID && g.UserID != nil && *g.UserID == userID {
			return g, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *fakeRepo) ListStudentsByGuardian(_ context.Context, tenantID, guardianID string) ([]*domain.Student, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Student
	for _, link := range r.links {
		if link.TenantID == tenantID && link.GuardianID == guardianID {
			if s, ok := r.students[link.StudentID]; ok && s.TenantID == tenantID {
				out = append(out, s)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (r *fakeRepo) List(_ context.Context, tenantID string, classID *string, limit int, cursor string) ([]*domain.Student, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Student
	for _, s := range r.students {
		if s.TenantID != tenantID {
			continue
		}
		if classID != nil && (s.ClassID == nil || *s.ClassID != *classID) {
			continue
		}
		out = append(out, s)
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

func (r *fakeRepo) ListStudentIDsByClassIDs(_ context.Context, tenantID string, classIDs []string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	allowed := make(map[string]struct{}, len(classIDs))
	for _, id := range classIDs {
		allowed[id] = struct{}{}
	}
	ids := []string{}
	for _, student := range r.students {
		if student.TenantID != tenantID || student.ClassID == nil || student.Status != string(domain.StatusActive) {
			continue
		}
		if _, ok := allowed[*student.ClassID]; ok {
			ids = append(ids, student.ID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (r *fakeRepo) Update(_ context.Context, tenantID string, s *domain.Student) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.students[s.ID]
	if !ok || existing.TenantID != tenantID {
		return domain.ErrNotFound
	}
	r.students[s.ID] = s
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, tenantID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.students[id]
	if !ok || existing.TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(r.students, id)
	return nil
}

func (r *fakeRepo) CreateGuardian(_ context.Context, tenantID string, g *domain.Guardian) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	g.TenantID = tenantID
	r.guardians[g.ID] = g
	return nil
}

func (r *fakeRepo) GetGuardianByID(_ context.Context, tenantID, id string) (*domain.Guardian, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	g, ok := r.guardians[id]
	if !ok || g.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return g, nil
}

func (r *fakeRepo) ListGuardiansByStudent(_ context.Context, tenantID, studentID string, limit int, _ string) ([]*domain.Guardian, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Guardian
	for _, link := range r.links {
		if link.TenantID == tenantID && link.StudentID == studentID {
			if g, ok := r.guardians[link.GuardianID]; ok {
				out = append(out, g)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		return out[:limit], out[limit-1].ID, nil
	}
	return out, "", nil
}

func (r *fakeRepo) UpdateGuardian(_ context.Context, tenantID string, g *domain.Guardian) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.guardians[g.ID]
	if !ok || existing.TenantID != tenantID {
		return domain.ErrNotFound
	}
	r.guardians[g.ID] = g
	return nil
}

func (r *fakeRepo) DeleteGuardian(_ context.Context, tenantID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.guardians[id]
	if !ok || existing.TenantID != tenantID {
		return domain.ErrNotFound
	}
	delete(r.guardians, id)
	return nil
}

func (r *fakeRepo) LinkGuardianToStudent(_ context.Context, tenantID string, link *domain.StudentGuardian) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	link.TenantID = tenantID
	r.links[link.ID] = link
	return nil
}

func (r *fakeRepo) UnlinkGuardianFromStudent(_ context.Context, tenantID, studentID, guardianID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, link := range r.links {
		if link.TenantID == tenantID && link.StudentID == studentID && link.GuardianID == guardianID {
			delete(r.links, k)
			return nil
		}
	}
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

func TestHandler_EnrollmentHistoryAndYearConflict(t *testing.T) {
	h, repo := newTestHandler()
	student, err := domain.NewStudent(tenantA, "Ama", "Mensah")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(context.Background(), tenantA, student); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h.Register(mux)
	body := map[string]any{
		"class_id":         "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		"academic_year_id": "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
	}
	create := httptest.NewRecorder()
	mux.ServeHTTP(create, request(t, http.MethodPost, "/api/v1/students/"+student.ID+"/enrollments", body, application.PermUpdate))
	if create.Code != http.StatusCreated {
		t.Fatalf("create enrollment: %d %s", create.Code, create.Body.String())
	}
	list := httptest.NewRecorder()
	mux.ServeHTTP(list, request(t, http.MethodGet, "/api/v1/students/"+student.ID+"/enrollments", nil, application.PermRead))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "academic_year_id") {
		t.Fatalf("list enrollments: %d %s", list.Code, list.Body.String())
	}
	duplicate := httptest.NewRecorder()
	mux.ServeHTTP(duplicate, request(t, http.MethodPost, "/api/v1/students/"+student.ID+"/enrollments", body, application.PermUpdate))
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate enrollment: %d %s", duplicate.Code, duplicate.Body.String())
	}
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
	req := httptest.NewRequestWithContext(context.Background(), method, path, bodyReader)
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
	s1, err := domain.NewStudent(tenantA, "A", "One")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s2, err := domain.NewStudent(tenantA, "B", "Two")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := repo.Create(ctx, tenantA, s1); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Create(ctx, tenantA, s2); err != nil {
		t.Fatalf("create: %v", err)
	}

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

func TestHandler_GetMyStudent(t *testing.T) {
	h, repo := newTestHandler()
	ctx := context.Background()
	userID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	student, err := domain.NewStudent(tenantA, "Ama", "Mensah")
	if err != nil {
		t.Fatalf("new student: %v", err)
	}
	student.UserID = &userID
	if err := repo.Create(ctx, tenantA, student); err != nil {
		t.Fatalf("create student: %v", err)
	}

	req := request(t, http.MethodGet, "/api/v1/students/me", nil, application.PermRead)
	req.Header.Set(auth.HeaderUserID, userID)
	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var got domain.Student
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != student.ID || got.UserID == nil || *got.UserID != userID {
		t.Fatalf("unexpected student: %+v", got)
	}
}

func TestHandler_GetMyStudent_UnlinkedUser(t *testing.T) {
	h, _ := newTestHandler()
	req := request(t, http.MethodGet, "/api/v1/students/me", nil, application.PermRead)
	req.Header.Set(auth.HeaderUserID, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
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

func TestHandler_ImportStudents(t *testing.T) {
	h, _ := newTestHandler()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "students.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	csv := `first_name,last_name,gender,guardian_first_name,guardian_last_name,guardian_email
Ada,Lovelace,female,Mother,Guardian,mother@example.com
Charles,Babbage,male,Father,Guardian,father@example.com`
	if _, err := part.Write([]byte(csv)); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := request(t, http.MethodPost, "/api/v1/students/import", nil, application.PermCreate)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Body = io.NopCloser(&body)

	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["students_created"] != float64(2) {
		t.Fatalf("expected 2 students, got %v", got)
	}
}

// Ensure fakeRepo error handling stays aligned with domain errors.
func TestFakeRepo_Isolation(t *testing.T) {
	repo := newFakeRepo()
	ctx := context.Background()
	s, err := domain.NewStudent(tenantA, "Tenant", "A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := repo.Create(ctx, tenantA, s); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.GetByID(ctx, "other-tenant", s.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for other tenant, got %v", err)
	}
}

func TestHandler_CreateAndLinkGuardian(t *testing.T) {
	h, _ := newTestHandler()
	req := request(t, http.MethodPost, "/api/v1/guardians", map[string]any{
		"first_name":   "Mother",
		"last_name":    "Guardian",
		"relationship": "mother",
		"phone":        "+233",
		"email":        "mother@example.com",
	}, application.PermCreate)
	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["relationship"] != "mother" {
		t.Fatalf("unexpected guardian: %v", got)
	}
}

func TestHandler_GetMyGuardianChildren(t *testing.T) {
	h, repo := newTestHandler()
	ctx := context.Background()
	userID := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"

	guardian, err := domain.NewGuardian(tenantA, "Efua", "Owusu", "mother")
	if err != nil {
		t.Fatalf("new guardian: %v", err)
	}
	guardian.UserID = &userID
	if err := repo.CreateGuardian(ctx, tenantA, guardian); err != nil {
		t.Fatalf("create guardian: %v", err)
	}
	student, err := domain.NewStudent(tenantA, "Kojo", "Owusu")
	if err != nil {
		t.Fatalf("new student: %v", err)
	}
	if err := repo.Create(ctx, tenantA, student); err != nil {
		t.Fatalf("create student: %v", err)
	}
	link, err := domain.NewStudentGuardian(tenantA, student.ID, guardian.ID, nil, true)
	if err != nil {
		t.Fatalf("new guardian link: %v", err)
	}
	if err := repo.LinkGuardianToStudent(ctx, tenantA, link); err != nil {
		t.Fatalf("link guardian: %v", err)
	}

	req := request(t, http.MethodGet, "/api/v1/guardians/me/children", nil, application.PermRead)
	req.Header.Set(auth.HeaderUserID, userID)
	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var got application.GuardianChildren
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Guardian == nil || got.Guardian.ID != guardian.ID {
		t.Fatalf("unexpected guardian: %+v", got.Guardian)
	}
	if len(got.Students) != 1 || got.Students[0].ID != student.ID {
		t.Fatalf("unexpected students: %+v", got.Students)
	}
}

func TestInternalLearnerScopeRequiresTokenAndReturnsOwnStudent(t *testing.T) {
	h, repo := newTestHandler()
	userID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	student := mustNewStudent(t, "Ama", "Mensah")
	student.UserID = &userID
	if err := repo.Create(context.Background(), tenantA, student); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h.RegisterInternal(mux, "internal-secret")
	unauthorized := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet,
		"/internal/v1/learner-scope?user_id="+userID+"&role=student", nil,
	)
	unauthorized.Header.Set(tenancy.HeaderTenantID, tenantA)
	denied := httptest.NewRecorder()
	mux.ServeHTTP(denied, unauthorized)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized=%d", denied.Code)
	}
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet,
		"/internal/v1/learner-scope?user_id="+userID+"&role=student", nil,
	)
	req.Header.Set("Authorization", "Bearer internal-secret")
	req.Header.Set(tenancy.HeaderTenantID, tenantA)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("scope=%d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		StudentIDs []string `json:"student_ids"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || len(body.StudentIDs) != 1 || body.StudentIDs[0] != student.ID {
		t.Fatalf("scope=%+v err=%v", body, err)
	}
}

type fixedClassScope struct{ classIDs []string }

func (s fixedClassScope) ResolveTeacherClasses(context.Context, string, string) ([]string, error) {
	return s.classIDs, nil
}

func TestInternalLearnerScopeReturnsOnlyAssignedTeacherRoster(t *testing.T) {
	repo := newFakeRepo()
	classOwn := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	classOther := "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	own := mustNewStudent(t, "Ama", "Assigned")
	own.ClassID = &classOwn
	other := mustNewStudent(t, "Kojo", "Other")
	other.ClassID = &classOther
	mustStoreStudent(t, repo, own)
	mustStoreStudent(t, repo, other)
	svc := application.NewService(repo, application.WithTeacherClassScopeResolver(fixedClassScope{classIDs: []string{classOwn}}))
	h := svchttp.NewHandler(svc)
	mux := http.NewServeMux()
	h.RegisterInternal(mux, "internal-secret")
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet,
		"/internal/v1/learner-scope?user_id=teacher-user&role=teacher", nil,
	)
	req.Header.Set("Authorization", "Bearer internal-secret")
	req.Header.Set(tenancy.HeaderTenantID, tenantA)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("scope=%d %s", rr.Code, rr.Body.String())
	}
	var scope application.ResolvedLearnerScope
	if err := json.Unmarshal(rr.Body.Bytes(), &scope); err != nil || len(scope.StudentIDs) != 1 || scope.StudentIDs[0] != own.ID || len(scope.ClassIDs) != 1 || scope.ClassIDs[0] != classOwn {
		t.Fatalf("scope=%+v err=%v", scope, err)
	}
}

func TestTeacherStudentReadsAreAssignedRosterOnly(t *testing.T) {
	repo := newFakeRepo()
	classOwn := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	classOther := "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	own := mustNewStudent(t, "Ama", "Assigned")
	own.ClassID = &classOwn
	other := mustNewStudent(t, "Kojo", "Other")
	other.ClassID = &classOther
	mustStoreStudent(t, repo, own)
	mustStoreStudent(t, repo, other)
	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureStudentManagement, true)
	svc := application.NewService(repo, application.WithFeatureGate(gates), application.WithTeacherClassScopeResolver(fixedClassScope{classIDs: []string{classOwn}}))
	teacher := auth.Actor{UserID: "teacher-user", TenantID: tenantA, Role: "teacher", Permissions: []string{application.PermRead}}
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantA})
	students, _, err := svc.List(ctx, teacher, nil, 20, "")
	if err != nil || len(students) != 1 || students[0].ID != own.ID {
		t.Fatalf("students=%+v err=%v", students, err)
	}
	if _, err := svc.Get(ctx, teacher, other.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unassigned get=%v", err)
	}
}
