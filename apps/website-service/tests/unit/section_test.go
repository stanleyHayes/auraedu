package unit

import (
	"testing"

	"github.com/auraedu/website-service/internal/domain"
)

func TestNewSection_RequiresTenant(t *testing.T) {
	if _, err := domain.NewSection("", "page-1", domain.SectionTypeHero, nil, 0); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewSection_RequiresPageID(t *testing.T) {
	if _, err := domain.NewSection("tenant-1", "", domain.SectionTypeHero, nil, 0); err == nil {
		t.Fatal("expected error when page_id is empty")
	}
}

func TestNewSection_RequiresValidType(t *testing.T) {
	if _, err := domain.NewSection("tenant-1", "page-1", domain.SectionType("invalid"), nil, 0); err == nil {
		t.Fatal("expected error for invalid section type")
	}
}

func TestNewSection_Valid(t *testing.T) {
	section, err := domain.NewSection("tenant-1", "page-1", domain.SectionTypeHero, domain.Content{"title": "Hero"}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if section.TenantID != "tenant-1" {
		t.Fatalf("tenant not set: got %q", section.TenantID)
	}
	if section.PageID != "page-1" {
		t.Fatalf("page_id not set: got %q", section.PageID)
	}
	if section.Status != string(domain.SectionStatusDraft) {
		t.Fatalf("expected draft status, got %q", section.Status)
	}
	if section.SortOrder != 1 {
		t.Fatalf("expected sort_order 1, got %d", section.SortOrder)
	}
}

func TestSection_ApplyUpdate(t *testing.T) {
	section, err := domain.NewSection("tenant-1", "page-1", domain.SectionTypeHero, domain.Content{"title": "Hero"}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sectionType := domain.SectionTypeText
	content := domain.Content{"body": "Hello"}
	order := 2
	status := string(domain.SectionStatusPublished)
	changed, err := section.ApplyUpdate(&sectionType, &content, &order, &status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 4 {
		t.Fatalf("expected 4 changed fields, got %v", changed)
	}
	if section.Type != string(sectionType) {
		t.Fatalf("type not updated: got %q", section.Type)
	}
	if section.SortOrder != order {
		t.Fatalf("sort_order not updated: got %d", section.SortOrder)
	}
}

func TestSection_InvalidStatus(t *testing.T) {
	section, err := domain.NewSection("tenant-1", "page-1", domain.SectionTypeHero, nil, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bad := "unknown"
	if _, err := section.ApplyUpdate(nil, nil, nil, &bad); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestSection_NegativeSortOrder(t *testing.T) {
	section, err := domain.NewSection("tenant-1", "page-1", domain.SectionTypeHero, nil, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	order := -1
	if _, err := section.ApplyUpdate(nil, nil, &order, nil); err == nil {
		t.Fatal("expected error for negative sort_order")
	}
}
