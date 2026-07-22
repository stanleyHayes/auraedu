package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunDistributesRequestsAcrossTenantsAndMeetsThresholds(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant := r.Header.Get("X-Tenant-ID")
		mu.Lock()
		seen[tenant]++
		mu.Unlock()
		if r.URL.Query().Get("tenant") != tenant {
			t.Errorf("query tenant=%q header tenant=%q", r.URL.Query().Get("tenant"), tenant)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		Name: "test", BaseURL: server.URL, Duration: duration{80 * time.Millisecond},
		Concurrency: 4, Timeout: duration{time.Second}, MaxErrorRate: 0,
		P95Millis: 500, P99Millis: 1000,
		Tenants:  []Tenant{{Code: "school-a"}, {Code: "school-b"}},
		Requests: []RequestSpec{{Name: "features", Method: http.MethodGet, Path: "/features?tenant={tenant}", ExpectedStatus: []int{200}}},
	}
	result, err := run(context.Background(), cfg)
	if err != nil || result.Requests == 0 || result.Failures != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if seen["school-a"] == 0 || seen["school-b"] == 0 {
		t.Fatalf("tenant distribution=%v", seen)
	}
}

func TestRunFailsThresholdOnUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	cfg := Config{
		Name: "failure", BaseURL: server.URL, Duration: duration{20 * time.Millisecond},
		Concurrency: 1, Timeout: duration{time.Second}, MaxErrorRate: 0,
		P95Millis: 500, P99Millis: 1000, Tenants: []Tenant{{Code: "school-a"}},
		Requests: []RequestSpec{{Name: "ready", Method: http.MethodGet, Path: "/ready", ExpectedStatus: []int{200}}},
	}
	if result, err := run(context.Background(), cfg); err == nil || result.Failures == 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestConfigRejectsDuplicateTenants(t *testing.T) {
	cfg := Config{Name: "duplicate", BaseURL: "https://example.test", Duration: duration{time.Second}, Concurrency: 1, Timeout: duration{time.Second}, MaxErrorRate: 0.01, P95Millis: 500, P99Millis: 1000, Tenants: []Tenant{{Code: "a"}, {Code: "a"}}, Requests: []RequestSpec{{Name: "ready", Path: "/ready", ExpectedStatus: []int{200}}}}
	if err := validate(cfg); err == nil {
		t.Fatal("duplicate tenants accepted")
	}
	if _, err := json.Marshal(cfg); err != nil {
		t.Fatalf("config marshal: %v", err)
	}
}

func TestRunFailsWhenArrivalRateIsNotSustained(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg := Config{
		Name: "under-capacity", BaseURL: server.URL, Duration: duration{80 * time.Millisecond},
		Concurrency: 1, RatePerSecond: 1000, MinThroughput: 900,
		Timeout: duration{time.Second}, MaxErrorRate: 0, P95Millis: 500, P99Millis: 1000,
		Tenants:  []Tenant{{Code: "school-a"}},
		Requests: []RequestSpec{{Name: "ready", Method: http.MethodGet, Path: "/ready", ExpectedStatus: []int{200}}},
	}
	result, err := run(context.Background(), cfg)
	if err == nil || result.Throughput >= cfg.MinThroughput {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestStagingExecutionRequiresAuditableProvenance(t *testing.T) {
	base := Config{
		Environment: "staging", BaseURL: "https://staging-api.auraedu.com",
		RunID: "release-2026-07-20", GitSHA: "abcdef1234567890",
		Tenants: []Tenant{{Code: "school-a", Token: "signed-token"}}, RequireAuth: true,
	}
	if err := validateExecution(base); err != nil {
		t.Fatalf("valid staging execution rejected: %v", err)
	}
	for name, mutate := range map[string]func(*Config){
		"placeholder host":  func(cfg *Config) { cfg.BaseURL = "https://staging.example" },
		"plaintext":         func(cfg *Config) { cfg.BaseURL = "http://staging-api.auraedu.com" },
		"missing run id":    func(cfg *Config) { cfg.RunID = "" },
		"missing git sha":   func(cfg *Config) { cfg.GitSHA = "" },
		"placeholder token": func(cfg *Config) { cfg.Tenants[0].Token = "replace-at-runtime" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			candidate.Tenants = append([]Tenant(nil), base.Tenants...)
			mutate(&candidate)
			if err := validateExecution(candidate); err == nil {
				t.Fatal("invalid staging execution accepted")
			}
		})
	}
}

func TestSummaryCarriesEvidenceWithoutCredentials(t *testing.T) {
	cfg := Config{
		Name: "evidence", Environment: "staging", BaseURL: "https://staging-api.auraedu.com",
		Duration: duration{time.Minute}, Concurrency: 4, RatePerSecond: 10, MinThroughput: 9,
		MaxErrorRate: 0.01, P95Millis: 750, P99Millis: 1500, TenantCount: 1,
		RunID: "release-2026-07-20", GitSHA: "abcdef1234567890",
		Tenants: []Tenant{{Code: "school-a", Token: "must-not-leak"}},
	}
	started := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	result := summarize(cfg, []sample{{Name: "ready", Tenant: "school-a", Duration: time.Millisecond}}, 2, started, started.Add(time.Second))
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "must-not-leak") {
		t.Fatal("result evidence contains tenant credential")
	}
	if result.ObservedTenantCount != 1 || result.RequiredTenantCount != 1 || result.Dropped != 2 || result.RunID == "" || result.GitSHA == "" {
		t.Fatalf("incomplete evidence summary: %+v", result)
	}
}

func TestPercentileUsesNearestRank(t *testing.T) {
	values := []time.Duration{time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond, 100 * time.Millisecond}
	if got := percentile(values, 0.95); got != 100*time.Millisecond {
		t.Fatalf("p95=%s", got)
	}
}

func TestResultEvidenceCannotOverwriteAnExistingArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.json")
	if err := writeResult(path, []byte("first\n")); err != nil {
		t.Fatal(err)
	}
	if err := writeResult(path, []byte("second\n")); err == nil {
		t.Fatal("existing result artifact was overwritten")
	}
	// #nosec G304 -- path is created inside this test's private temporary directory.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first\n" {
		t.Fatalf("artifact=%q", data)
	}
}
