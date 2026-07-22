package file

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auraedu/admissions-service/internal/domain"
	"github.com/auraedu/platform/tenancy"
)

func TestVerifyOwnership(t *testing.T) {
	const fileID = "62d564d4-428f-4a4a-851d-f428f7766358"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get(tenancy.HeaderTenantID) != "school-one" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprintf(
			w,
			`{"file_id":%q,"owner_id":"applicant-1","status":"active","content_type":"application/pdf"}`,
			fileID,
		); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()
	if err := NewClient(server.URL, "secret").Verify(context.Background(), "school-one", "applicant-1", fileID); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRejectsUnsafeMetadataAndUnavailableDependency(t *testing.T) {
	const fileID = "62d564d4-428f-4a4a-851d-f428f7766358"
	for name, body := range map[string]string{
		"wrong owner":   `{"file_id":"` + fileID + `","owner_id":"someone-else","status":"active","content_type":"application/pdf"}`,
		"unsafe type":   `{"file_id":"` + fileID + `","owner_id":"applicant-1","status":"active","content_type":"text/html"}`,
		"inactive file": `{"file_id":"` + fileID + `","owner_id":"applicant-1","status":"deleted","content_type":"application/pdf"}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			err := NewClient(server.URL, "secret").Verify(context.Background(), "school-one", "applicant-1", fileID)
			if !errors.Is(err, domain.ErrForbidden) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if err := NewClient("", "").Verify(context.Background(), "school-one", "applicant-1", fileID); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("unconfigured dependency=%v", err)
	}
}
