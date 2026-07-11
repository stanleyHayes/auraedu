package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNewCloudinaryStorage_RequiresURL(t *testing.T) {
	if _, err := NewCloudinaryStorage(""); err == nil {
		t.Fatal("expected error for empty cloudinary URL")
	}
}

func TestNewCloudinaryStorage_InvalidURL(t *testing.T) {
	if _, err := NewCloudinaryStorage("not-a-url"); err == nil {
		t.Fatal("expected error for invalid cloudinary URL")
	}
}

func TestCloudinaryStorage_Open(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantPrefix := "/raw/upload/tenant-1/file-1"
		if !strings.HasPrefix(r.URL.Path, wantPrefix) {
			t.Errorf("path: got %q, want prefix %q", r.URL.Path, wantPrefix)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello cloudinary"))
	}))
	defer server.Close()

	store, err := NewCloudinaryStorage("cloudinary://key:secret@testcloud",
		WithDeliveryBaseURL(server.URL+"/raw/upload"),
	)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}

	rc, err := store.Open(context.Background(), "tenant-1", "tenant-1/file-1")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer rc.Close()

	body, _ := io.ReadAll(rc)
	if string(body) != "hello cloudinary" {
		t.Fatalf("body: got %q", string(body))
	}
}

func TestCloudinaryStorage_OpenNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	store, err := NewCloudinaryStorage("cloudinary://key:secret@testcloud",
		WithDeliveryBaseURL(server.URL+"/raw/upload"),
	)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}

	_, err = store.Open(context.Background(), "tenant-1", "tenant-1/file-1")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestCloudinaryStorage_DeleteEmptyPath(t *testing.T) {
	store, err := NewCloudinaryStorage("cloudinary://key:secret@testcloud")
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	if err := store.Delete(context.Background(), "tenant-1", ""); err != nil {
		t.Fatalf("delete empty path: %v", err)
	}
}

func TestCloudinaryStorage_SaveRequiresIDs(t *testing.T) {
	store, err := NewCloudinaryStorage("cloudinary://key:secret@testcloud")
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	if _, err := store.Save(context.Background(), "", "file-1", strings.NewReader("x")); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
	if _, err := store.Save(context.Background(), "tenant-1", "", strings.NewReader("x")); err == nil {
		t.Fatal("expected error when file_id is empty")
	}
}

func TestCloudinaryStorage_SignUpload(t *testing.T) {
	store, err := NewCloudinaryStorage("cloudinary://key:secret@testcloud")
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}

	signed, err := store.SignUpload(context.Background(), "tenant-1", "file-1", "tenant-1/documents", "raw")
	if err != nil {
		t.Fatalf("sign upload: %v", err)
	}
	if signed.Signature == "" {
		t.Fatal("expected non-empty signature")
	}
	if signed.Timestamp == 0 {
		t.Fatal("expected non-zero timestamp")
	}
	if signed.APIKey != "key" {
		t.Fatalf("api key: got %q, want key", signed.APIKey)
	}
	if signed.CloudName != "testcloud" {
		t.Fatalf("cloud name: got %q, want testcloud", signed.CloudName)
	}
	if signed.PublicID != "tenant-1/documents/file-1" {
		t.Fatalf("public id: got %q", signed.PublicID)
	}
	wantURL := "https://api.cloudinary.com/v1_1/testcloud/raw/upload"
	if signed.UploadURL != wantURL {
		t.Fatalf("upload url: got %q, want %q", signed.UploadURL, wantURL)
	}
}

func TestCloudinaryStorage_SignUploadDefaultsResourceType(t *testing.T) {
	store, err := NewCloudinaryStorage("cloudinary://key:secret@testcloud")
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}

	signed, err := store.SignUpload(context.Background(), "tenant-1", "file-1", "tenant-1/docs", "")
	if err != nil {
		t.Fatalf("sign upload: %v", err)
	}
	if signed.ResourceType != "raw" {
		t.Fatalf("resource type: got %q, want raw", signed.ResourceType)
	}
}

func TestCloudinaryStorage_SignUploadRequiresIDs(t *testing.T) {
	store, err := NewCloudinaryStorage("cloudinary://key:secret@testcloud")
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	if _, err := store.SignUpload(context.Background(), "", "file-1", "folder", "raw"); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
	if _, err := store.SignUpload(context.Background(), "tenant-1", "", "folder", "raw"); err == nil {
		t.Fatal("expected error when file_id is empty")
	}
}

func TestCloudinaryStorage_DeliveryURL(t *testing.T) {
	store, err := NewCloudinaryStorage("cloudinary://key:secret@testcloud",
		WithResourceType("image"),
	)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}

	url, err := store.DeliveryURL("tenant-1", "tenant-1/file-1", "", "w_300,h_300,c_fit")
	if err != nil {
		t.Fatalf("delivery url: %v", err)
	}
	want := "https://res.cloudinary.com/testcloud/image/w_300,h_300,c_fit/upload/tenant-1/file-1"
	if url != want {
		t.Fatalf("url: got %q, want %q", url, want)
	}

	url, err = store.DeliveryURL("tenant-1", "tenant-1/file-1", "", "")
	if err != nil {
		t.Fatalf("delivery url no transform: %v", err)
	}
	want = "https://res.cloudinary.com/testcloud/image/upload/tenant-1/file-1"
	if url != want {
		t.Fatalf("url: got %q, want %q", url, want)
	}
}

func TestCloudinaryStorage_DeliveryURLEmptyPath(t *testing.T) {
	store, err := NewCloudinaryStorage("cloudinary://key:secret@testcloud")
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	if _, err := store.DeliveryURL("tenant-1", "", "", ""); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestCloudinaryStorage_VerifyWebhook(t *testing.T) {
	store, err := NewCloudinaryStorage("cloudinary://key:secret@testcloud")
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}

	body := []byte(`{"public_id":"tenant-1/file-1","secure_url":"https://example.com/x"}`)
	timestamp := time.Now().Unix()
	payload := strconv.FormatInt(timestamp, 10) + string(body)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))

	if !store.VerifyWebhook(timestamp, signature, body) {
		t.Fatal("expected valid SHA-256 webhook signature to verify")
	}

	if store.VerifyWebhook(timestamp, "bad-signature", body) {
		t.Fatal("expected invalid signature to fail")
	}

	if store.VerifyWebhook(timestamp, signature, []byte(`{}`)) {
		t.Fatal("expected body mismatch to fail")
	}
}
