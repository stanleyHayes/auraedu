package storage

import (
	"context"
	"io"
	"path/filepath"
	"testing"
)

func TestLocalStorageTenantConfinement(t *testing.T) {
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path, err := store.Save(context.Background(), "school-a", "card.pdf", []byte("pdf"))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := store.Open(context.Background(), "school-a", path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stored report: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stored report: %v", err)
	}
	if string(got) != "pdf" {
		t.Fatalf("stored content = %q", got)
	}
	for _, unsafe := range []string{"../secret", filepath.Join("..", "school-b", "card.pdf"), "/tmp/card.pdf"} {
		if _, err := store.Open(context.Background(), "school-a", unsafe); err == nil {
			t.Fatalf("expected tenant escape rejection for %q", unsafe)
		}
	}
}

func TestCloudinaryKeyTenantConfinement(t *testing.T) {
	key, err := cloudinaryKey("school-a", "card.pdf")
	if err != nil || key != "reports/school-a/card.pdf" {
		t.Fatalf("key=%q err=%v", key, err)
	}
	if _, err := cloudinaryKey("school-a", "reports/school-b/card.pdf"); err == nil {
		t.Fatal("expected cross-tenant Cloudinary key rejection")
	}
	if _, err := cloudinaryKey("school-a", "../card.pdf"); err == nil {
		t.Fatal("expected traversal rejection")
	}
}
