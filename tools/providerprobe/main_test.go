package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testProviderConfig(baseURL string) Config {
	return Config{
		Name: "auraedu-staging-email-provider", Environment: "staging", BaseURL: baseURL,
		Timeout: duration{2 * time.Second}, DeliveryTimeout: duration{10 * time.Second}, PollInterval: duration{250 * time.Millisecond},
		RunID: "release-provider-test", GitSHA: "abcdef1234567890", Tenant: "school-a",
		Token: "runtime-bearer-token-123456", RecipientID: "0198f0db-7d3d-7000-8000-000000000001", Email: "release@example.test",
	}
}

func TestRunProvesProviderAcceptanceWithoutRetainingSecrets(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var fetches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer runtime-bearer-token-123456" || r.Header.Get("X-Tenant-Code") != "school-a" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/messages":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"0198f0db-7d3d-7000-8000-000000000002","tenant_id":"school-a","channel":"email","status":"pending"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/send"):
			_, _ = w.Write([]byte(`{"id":"0198f0db-7d3d-7000-8000-000000000002","tenant_id":"school-a","channel":"email","status":"sent","sent_at":"` + now + `","provider":"resend","delivery_status":"accepted","delivery_status_at":"` + now + `"}`))
		case r.Method == http.MethodGet:
			status := "accepted"
			if fetches.Add(1) > 1 {
				status = "delivered"
			}
			_, _ = w.Write([]byte(`{"id":"0198f0db-7d3d-7000-8000-000000000002","tenant_id":"school-a","channel":"email","status":"sent","sent_at":"` + now + `","provider":"resend","delivery_status":"` + status + `","delivery_status_at":"` + now + `"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	evidence, err := run(context.Background(), testProviderConfig(server.URL))
	if err != nil || !evidence.AllPassed || evidence.Provider != "resend" || evidence.ProviderOutcome != "accepted" ||
		evidence.DeliveryStatus != "delivered" || evidence.DeliveredAt == nil || len(evidence.Steps) != 4 {
		t.Fatalf("evidence=%+v err=%v", evidence, err)
	}
	encoded, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"runtime-bearer-token-123456", "school-a", "release@example.test", "0198f0db-7d3d-7000-8000-000000000002"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("evidence leaked %q", secret)
		}
	}
}

func TestRunRejectsAcceptanceWithoutDeliveredFeedback(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/messages" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"0198f0db-7d3d-7000-8000-000000000002","channel":"email","status":"pending"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"0198f0db-7d3d-7000-8000-000000000002","channel":"email","status":"sent","sent_at":"` + now + `","provider":"resend","delivery_status":"accepted","delivery_status_at":"` + now + `"}`))
	}))
	defer server.Close()
	cfg := testProviderConfig(server.URL)
	cfg.DeliveryTimeout = duration{20 * time.Millisecond}
	evidence, err := run(context.Background(), cfg)
	if err == nil || evidence.AllPassed || evidence.DeliveryStatus != "accepted" || len(evidence.Steps) != 4 {
		t.Fatalf("undelivered provider outcome accepted: evidence=%+v err=%v", evidence, err)
	}
}

func TestRunRejectsAnUnpersistedProviderOutcome(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/messages" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"0198f0db-7d3d-7000-8000-000000000002","channel":"email","status":"pending"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"0198f0db-7d3d-7000-8000-000000000002","channel":"email","status":"pending"}`))
	}))
	defer server.Close()
	evidence, err := run(context.Background(), testProviderConfig(server.URL))
	if err == nil || evidence.AllPassed || evidence.ProviderOutcome == "accepted" {
		t.Fatalf("unpersisted outcome accepted: evidence=%+v err=%v", evidence, err)
	}
}

func TestExecutionRejectsPlaceholderTargetsAndCredentials(t *testing.T) {
	cfg := testProviderConfig("https://staging-api.auraedu.com")
	if err := validateExecution(cfg); err != nil {
		t.Fatal(err)
	}
	cfg.BaseURL = "https://staging.example"
	if err := validateExecution(cfg); err == nil {
		t.Fatal("placeholder origin accepted")
	}
	cfg.BaseURL = "https://staging-api.auraedu.com"
	cfg.Token = "replace-at-runtime-token"
	if err := validateExecution(cfg); err == nil {
		t.Fatal("placeholder token accepted")
	}
}
