package unit

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	svchttp "github.com/auraedu/student-service/internal/adapters/http"
	"github.com/auraedu/student-service/internal/application"
	"github.com/auraedu/student-service/internal/domain"
)

// Fixed UUIDs for the class-roster filter tests (AURA-10.11).
const (
	classX = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	classY = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	yearY  = "cccccccc-cccc-cccc-cccc-cccccccccccc"

	tenantB = "22222222-2222-2222-2222-222222222222"
)

func ptr(s string) *string { return &s }

// newClassFilterHandler enables the feature flag for both tenants so cross-tenant
// behavior can be exercised.
func newClassFilterHandler() (*svchttp.Handler, *fakeRepo) {
	repo := newFakeRepo()
	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureStudentManagement, true)
	gates.Set(tenantB, application.FeatureStudentManagement, true)
	svc := application.NewService(repo, application.WithFeatureGate(gates))
	return svchttp.NewHandler(svc), repo
}

func seedStudent(t *testing.T, repo *fakeRepo, tenantID, first, last string, classID, yearID *string) *domain.Student {
	t.Helper()
	s, err := domain.NewStudent(tenantID, first, last)
	if err != nil {
		t.Fatalf("new student: %v", err)
	}
	s.ClassID = classID
	s.AcademicYearID = yearID
	if err := repo.Create(context.Background(), tenantID, s); err != nil {
		t.Fatalf("create student: %v", err)
	}
	return s
}

// requestAs is request() with an explicit tenant (the shared helper pins tenantA).
func requestAs(t *testing.T, tenantID, method, path string, perms ...string) *http.Request {
	t.Helper()
	req := request(t, method, path, nil, perms...)
	req.Header.Set(tenancy.HeaderTenantID, tenantID)
	req.Header.Set(auth.HeaderTenant, tenantID)
	return req
}

func serveList(t *testing.T, h *svchttp.Handler, req *http.Request) (int, map[string]any) {
	t.Helper()
	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(rr, req)
	var got map[string]any
	if rr.Code == http.StatusOK {
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return rr.Code, got
}

func listIDs(t *testing.T, got map[string]any) []string {
	t.Helper()
	data, ok := got["data"].([]any)
	if !ok {
		t.Fatalf("missing data array: %v", got)
	}
	ids := make([]string, 0, len(data))
	for _, item := range data {
		m, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("unexpected item: %v", item)
		}
		ids = append(ids, m["id"].(string))
	}
	return ids
}

func TestHandler_ListStudents_FilterByClass(t *testing.T) {
	h, repo := newClassFilterHandler()
	s1 := seedStudent(t, repo, tenantA, "Ama", "One", ptr(classX), ptr(yearY))
	seedStudent(t, repo, tenantA, "Kojo", "Two", ptr(classY), nil)
	seedStudent(t, repo, tenantA, "Esi", "Three", nil, nil)

	// Filter set: only the roster of classX.
	code, got := serveList(t, h, requestAs(t, tenantA, http.MethodGet, "/api/v1/students?class_id="+classX, application.PermRead))
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	ids := listIDs(t, got)
	if len(ids) != 1 || ids[0] != s1.ID {
		t.Fatalf("expected only student %s, got %v", s1.ID, ids)
	}
	row := got["data"].([]any)[0].(map[string]any)
	if row["class_id"] != classX || row["academic_year_id"] != yearY {
		t.Fatalf("DTO missing class fields: %v", row)
	}

	// Filter set to the other class.
	_, got = serveList(t, h, requestAs(t, tenantA, http.MethodGet, "/api/v1/students?class_id="+classY, application.PermRead))
	if ids := listIDs(t, got); len(ids) != 1 {
		t.Fatalf("expected 1 student for classY, got %v", ids)
	}

	// Filter unset: every student of the tenant.
	_, got = serveList(t, h, requestAs(t, tenantA, http.MethodGet, "/api/v1/students", application.PermRead))
	if ids := listIDs(t, got); len(ids) != 3 {
		t.Fatalf("expected 3 students without filter, got %v", ids)
	}
}

func TestHandler_ListStudents_FilterCrossTenant(t *testing.T) {
	h, repo := newClassFilterHandler()
	seedStudent(t, repo, tenantA, "Tenant", "A", ptr(classX), nil)
	sB := seedStudent(t, repo, tenantB, "Tenant", "B", ptr(classX), nil)

	// Same class_id value, different tenant: the filter must stay tenant-scoped.
	code, got := serveList(t, h, requestAs(t, tenantB, http.MethodGet, "/api/v1/students?class_id="+classX, application.PermRead))
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	ids := listIDs(t, got)
	if len(ids) != 1 || ids[0] != sB.ID {
		t.Fatalf("tenant B should see only its own roster, got %v", ids)
	}

	// And with the filter unset tenant B must not see tenant A students.
	_, got = serveList(t, h, requestAs(t, tenantB, http.MethodGet, "/api/v1/students", application.PermRead))
	if ids := listIDs(t, got); len(ids) != 1 || ids[0] != sB.ID {
		t.Fatalf("tenant B should see only its own students, got %v", ids)
	}
}

func TestHandler_ListStudents_InvalidClassID(t *testing.T) {
	h, _ := newClassFilterHandler()
	code, _ := serveList(t, h, requestAs(t, tenantA, http.MethodGet, "/api/v1/students?class_id=not-a-uuid", application.PermRead))
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for non-UUID class_id, got %d", code)
	}
}

func TestHandler_CreateStudent_PersistsClassFields(t *testing.T) {
	h, _ := newClassFilterHandler()
	req := request(t, http.MethodPost, "/api/v1/students", map[string]any{
		"first_name":       "Kwame",
		"last_name":        "Nkrumah",
		"class_id":         classX,
		"academic_year_id": yearY,
	}, application.PermCreate, application.PermRead)
	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created["class_id"] != classX || created["academic_year_id"] != yearY {
		t.Fatalf("create response missing class fields: %v", created)
	}

	// The fields must round-trip through the read DTO as well.
	code, got := serveList(t, h, requestAs(t, tenantA, http.MethodGet, "/api/v1/students?class_id="+classX, application.PermRead))
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if ids := listIDs(t, got); len(ids) != 1 || ids[0] != created["id"] {
		t.Fatalf("expected created student in roster, got %v", ids)
	}
}

func TestHandler_CreateStudent_InvalidClassID(t *testing.T) {
	h, _ := newClassFilterHandler()
	req := request(t, http.MethodPost, "/api/v1/students", map[string]any{
		"first_name": "X",
		"last_name":  "Y",
		"class_id":   "not-a-uuid",
	}, application.PermCreate)
	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for non-UUID class_id, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestService_List_ClassIDNormalization(t *testing.T) {
	repo := newFakeRepo()
	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureStudentManagement, true)
	svc := application.NewService(repo, application.WithFeatureGate(gates))
	seedStudent(t, repo, tenantA, "Ama", "One", ptr(classX), nil)

	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantA})
	actor := auth.Actor{UserID: "user-1", TenantID: tenantA, Permissions: []string{application.PermRead}}

	// Empty string is treated as unset, not as a filter on empty class_id.
	all, _, err := svc.List(ctx, actor, ptr(""), 10, "")
	if err != nil {
		t.Fatalf("list with empty class_id: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected empty class_id to disable the filter, got %d students", len(all))
	}

	// Non-UUID class_id is a validation error.
	if _, _, err := svc.List(ctx, actor, ptr("nope"), 10, ""); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestStudent_ClassFieldValidation(t *testing.T) {
	s, err := domain.NewStudent("tenant-1", "Ada", "Lovelace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.ClassID = ptr("not-a-uuid")
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for invalid class_id")
	}

	s.ClassID = ptr(classX)
	s.AcademicYearID = ptr("also-not-a-uuid")
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for invalid academic_year_id")
	}

	s.AcademicYearID = ptr(yearY)
	if err := s.Validate(); err != nil {
		t.Fatalf("unexpected error for valid class fields: %v", err)
	}
}

// The list endpoint must ignore a blank class_id rather than 422 on it.
func TestHandler_ListStudents_BlankClassID(t *testing.T) {
	h, repo := newClassFilterHandler()
	seedStudent(t, repo, tenantA, "Ama", "One", ptr(classX), nil)

	code, got := serveList(t, h, requestAs(t, tenantA, http.MethodGet, "/api/v1/students?class_id=", application.PermRead))
	if code != http.StatusOK {
		t.Fatalf("expected 200 for blank class_id, got %d", code)
	}
	if ids := listIDs(t, got); len(ids) != 1 {
		t.Fatalf("expected blank class_id to disable the filter, got %v", ids)
	}
}
