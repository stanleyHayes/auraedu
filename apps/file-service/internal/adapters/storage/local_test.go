package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorageRejectsCrossTenantAndTraversalPaths(t *testing.T) {
	store := NewLocalStorage(t.TempDir())
	path, err := store.Save(context.Background(), "school-a", "file-1", bytes.NewBufferString("safe"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := store.Open(context.Background(), "school-b", path); err == nil {
		t.Fatal("school B opened school A path")
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if _, err := store.Open(context.Background(), "school-a", outside); err == nil {
		t.Fatal("absolute traversal path was accepted")
	}
	if err := store.Delete(context.Background(), "school-a", outside); err == nil {
		t.Fatal("delete traversal path was accepted")
	}
	data, err := os.ReadFile(outside) //nolint:gosec // The test deliberately verifies a temporary traversal target remains unchanged.
	if err != nil || string(data) != "secret" {
		t.Fatalf("outside file changed: data=%q err=%v", data, err)
	}
	rc, err := store.Open(context.Background(), "school-a", path)
	if err != nil {
		t.Fatalf("open own file: %v", err)
	}
	defer func() {
		if closeErr := rc.Close(); closeErr != nil {
			t.Errorf("close stored file: %v", closeErr)
		}
	}()
	data, err = io.ReadAll(rc)
	if err != nil || string(data) != "safe" {
		t.Fatalf("own file data=%q err=%v", data, err)
	}
}
