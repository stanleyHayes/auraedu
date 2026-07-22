package unit

import (
	"testing"

	"github.com/auraedu/file-service/internal/domain"
)

func TestNewFileUpload_RequiresTenant(t *testing.T) {
	if _, err := domain.NewFileUpload("", "notes.pdf", "application/pdf", "user-1", "document", 100, "abc"); err == nil {
		t.Fatal("expected error when tenant_id is empty (tenant isolation invariant)")
	}
}

func TestNewFileUpload_RequiresFilename(t *testing.T) {
	if _, err := domain.NewFileUpload("tenant-1", "", "application/pdf", "user-1", "document", 100, "abc"); err == nil {
		t.Fatal("expected error when original_filename is empty")
	}
	if _, err := domain.NewFileUpload("tenant-1", "   ", "application/pdf", "user-1", "document", 100, "abc"); err == nil {
		t.Fatal("expected error when original_filename is whitespace")
	}
}

func TestNewFileUpload_SanitizesPathAndRejectsHeaderInjection(t *testing.T) {
	file, err := domain.NewFileUpload("tenant-1", `C:\\fakepath\\report.pdf`, "application/pdf", "user-1", "document", 100, "abc")
	if err != nil {
		t.Fatalf("sanitize browser path: %v", err)
	}
	if file.OriginalFilename != "report.pdf" {
		t.Fatalf("filename=%q", file.OriginalFilename)
	}
	if _, err := domain.NewFileUpload("tenant-1", "report.pdf\r\nX-Evil: yes", "application/pdf", "user-1", "document", 100, "abc"); err == nil {
		t.Fatal("expected control-character filename to be rejected")
	}
}

func TestNewFileUpload_RequiresNonNegativeSize(t *testing.T) {
	e, err := domain.NewFileUpload("tenant-1", "notes.pdf", "application/pdf", "user-1", "document", -1, "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := e.Validate(); err == nil {
		t.Fatal("expected validation error for negative size_bytes")
	}
}

func TestNewFileUpload_Valid(t *testing.T) {
	e, err := domain.NewFileUpload("tenant-1", "notes.pdf", "application/pdf", "user-1", "document", 1024, "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.TenantID != "tenant-1" {
		t.Fatalf("tenant not set: got %q", e.TenantID)
	}
	if e.OriginalFilename != "notes.pdf" {
		t.Fatalf("filename not set: got %q", e.OriginalFilename)
	}
	if e.ContentType != "application/pdf" {
		t.Fatalf("content type not set: got %q", e.ContentType)
	}
	if e.Status != string(domain.StatusPending) {
		t.Fatalf("expected pending status, got %q", e.Status)
	}
	if e.OwnerID != "user-1" {
		t.Fatalf("owner not set: got %q", e.OwnerID)
	}
}

func TestFileUpload_ApplyUpdate(t *testing.T) {
	e, err := domain.NewFileUpload("tenant-1", "notes.pdf", "application/pdf", "user-1", "document", 1024, "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	name := "renamed.pdf"
	status := string(domain.StatusArchived)
	changed, err := e.ApplyUpdate(&name, nil, nil, &status, map[string]any{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 3 {
		t.Fatalf("expected 3 changed fields, got %v", changed)
	}
	if e.OriginalFilename != name {
		t.Fatalf("filename not updated: got %q", e.OriginalFilename)
	}
	if e.Status != status {
		t.Fatalf("status not updated: got %q", e.Status)
	}
	if e.Metadata["key"] != "value" {
		t.Fatalf("metadata not updated: %+v", e.Metadata)
	}
}

func TestFileUpload_InvalidStatus(t *testing.T) {
	e, err := domain.NewFileUpload("tenant-1", "notes.pdf", "application/pdf", "user-1", "document", 1024, "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bad := "unknown"
	if _, err := e.ApplyUpdate(nil, nil, nil, &bad, nil); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestComputeChecksum(t *testing.T) {
	got := domain.ComputeChecksum([]byte("hello"))
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Fatalf("checksum mismatch: got %q want %q", got, want)
	}
}

func TestLocalStoragePath(t *testing.T) {
	e, err := domain.NewFileUpload("tenant-1", "notes.pdf", "application/pdf", "user-1", "document", 1024, "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	path := e.LocalStoragePath("/tmp/files")
	if path != "/tmp/files/tenant-1/"+e.ID {
		t.Fatalf("unexpected storage path: %q", path)
	}
}
