package unit

import (
	"testing"

	"github.com/auraedu/website-service/internal/domain"
)

func TestNewPage_RequiresTenant(t *testing.T) {
	if _, err := domain.NewPage("", "home", "Home"); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewPage_RequiresSlug(t *testing.T) {
	if _, err := domain.NewPage("tenant-1", "", "Home"); err == nil {
		t.Fatal("expected error when slug is empty")
	}
}

func TestNewPage_RequiresTitle(t *testing.T) {
	if _, err := domain.NewPage("tenant-1", "home", ""); err == nil {
		t.Fatal("expected error when title is empty")
	}
}

func TestNewPage_Valid(t *testing.T) {
	page, err := domain.NewPage("tenant-1", "home", "Home")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.TenantID != "tenant-1" {
		t.Fatalf("tenant not set: got %q", page.TenantID)
	}
	if page.Slug != "home" {
		t.Fatalf("slug not set: got %q", page.Slug)
	}
	if page.Status != string(domain.PageStatusDraft) {
		t.Fatalf("expected draft status, got %q", page.Status)
	}
	if page.Layout != string(domain.PageLayoutDefault) {
		t.Fatalf("expected default layout, got %q", page.Layout)
	}
}

func TestPage_ApplyUpdate(t *testing.T) {
	page, err := domain.NewPage("tenant-1", "home", "Home")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	title := "Welcome"
	layout := string(domain.PageLayoutLanding)
	status := string(domain.PageStatusPublished)
	changed, err := page.ApplyUpdate(nil, &title, &status, nil, &layout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 3 {
		t.Fatalf("expected 3 changed fields, got %v", changed)
	}
	if page.Title != title {
		t.Fatalf("title not updated: got %q", page.Title)
	}
	if page.Layout != layout {
		t.Fatalf("layout not updated: got %q", page.Layout)
	}
	if page.Status != status {
		t.Fatalf("status not updated: got %q", page.Status)
	}
	if page.PublishedAt == nil {
		t.Fatal("expected published_at to be set")
	}
}

func TestPage_InvalidStatus(t *testing.T) {
	page, err := domain.NewPage("tenant-1", "home", "Home")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bad := "unknown"
	if _, err := page.ApplyUpdate(nil, nil, &bad, nil, nil); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestPage_InvalidLayout(t *testing.T) {
	page, err := domain.NewPage("tenant-1", "home", "Home")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bad := "unknown"
	if _, err := page.ApplyUpdate(nil, nil, nil, nil, &bad); err == nil {
		t.Fatal("expected error for invalid layout")
	}
}

func TestPage_NormalizesSlug(t *testing.T) {
	page, err := domain.NewPage("tenant-1", "  About Us  ", "About Us")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Slug != "about-us" {
		t.Fatalf("expected normalized slug, got %q", page.Slug)
	}
}
