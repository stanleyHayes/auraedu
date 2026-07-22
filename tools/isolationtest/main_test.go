package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testConfig(serverURL string) Config {
	return Config{
		Name: "test-isolation", Environment: "staging", BaseURL: serverURL,
		Timeout: duration{time.Second}, MinimumProbeCount: 8,
		RunID: "release-test-run", GitSHA: "abcdef1234567890",
		Schools: []School{
			{Code: "school-a", Token: "school-a-runtime-bearer-123456", Resources: map[string]string{}},
			{Code: "school-b", Token: "school-b-runtime-bearer-123456", Resources: map[string]string{}},
		},
	}
}

func addTestProbes(cfg *Config) {
	for index := 1; index <= 8; index++ {
		key := "resource-" + string(rune('a'+index-1))
		cfg.Schools[0].Resources[key] = "school-a-" + key
		cfg.Schools[1].Resources[key] = "school-b-" + key
		cfg.Probes = append(cfg.Probes, Probe{
			Name: key, Path: "/api/v1/resources/{resource}", ResourceKey: key,
			OwnStatus: []int{200}, CrossStatus: []int{404}, MismatchStatus: []int{403},
		})
	}
}

func TestRunProvesBothDirectionsWithoutLeakingSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenTenant := ""
		switch r.Header.Get("Authorization") {
		case "Bearer school-a-runtime-bearer-123456":
			tokenTenant = "a"
		case "Bearer school-b-runtime-bearer-123456":
			tokenTenant = "b"
		}
		headerTenant := strings.TrimPrefix(r.Header.Get("X-Tenant-Code"), "school-")
		if tokenTenant != headerTenant {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"code":"tenant_mismatch"}`))
			return
		}
		if strings.Contains(r.URL.Path, "school-"+tokenTenant+"-") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"owned"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"not_found"}`))
	}))
	defer server.Close()
	cfg := testConfig(server.URL)
	addTestProbes(&cfg)
	if err := validate(cfg); err != nil {
		t.Fatal(err)
	}
	evidence, err := run(context.Background(), cfg)
	if err != nil || !evidence.AllPassed || evidence.PassedChecks != 48 || evidence.FailedChecks != 0 {
		t.Fatalf("evidence=%+v err=%v", evidence, err)
	}
	encoded, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"school-a-runtime-bearer-123456", "school-b-runtime-bearer-123456", "school-a-resource-a", "school-b-resource-a"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("evidence leaked %q", secret)
		}
	}
}

func TestRunFailsA200CrossTenantLeak(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg := testConfig(server.URL)
	addTestProbes(&cfg)
	evidence, err := run(context.Background(), cfg)
	if err == nil || evidence.FailedChecks == 0 || evidence.AllPassed {
		t.Fatalf("evidence=%+v err=%v", evidence, err)
	}
}

func TestRunFailsWhenADenialBodyDisclosesTheOtherSchool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "school-b-resource-a") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"debug":"school-b-resource-a"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg := testConfig(server.URL)
	addTestProbes(&cfg)
	evidence, err := run(context.Background(), cfg)
	if err == nil || evidence.AllPassed {
		t.Fatalf("disclosing denial accepted: evidence=%+v err=%v", evidence, err)
	}
	found := false
	for _, check := range evidence.Checks {
		if check.Failure == "denial_response_disclosed_tenant_or_resource" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("disclosing denial did not produce the expected sanitized failure")
	}
}

func TestWriteEvidenceCannotOverwriteAnExistingArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.json")
	if err := writeEvidence(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := writeEvidence(path, []byte("second")); err == nil {
		t.Fatal("existing evidence was overwritten")
	}
	// #nosec G304 -- path is created inside this test's private temporary directory.
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "first" {
		t.Fatalf("evidence changed: data=%q err=%v", data, err)
	}
}

func TestExecutionRejectsPlaceholdersAndMissingProvenance(t *testing.T) {
	cfg := testConfig("https://staging-api.auraedu.com")
	addTestProbes(&cfg)
	if err := validateExecution(cfg); err != nil {
		t.Fatalf("valid execution rejected: %v", err)
	}
	cfg.BaseURL = "https://staging.example"
	if err := validateExecution(cfg); err == nil {
		t.Fatal("placeholder host accepted")
	}
	cfg.BaseURL = "https://staging-api.auraedu.com"
	cfg.Schools[0].Token = "replace-at-runtime"
	if err := validateExecution(cfg); err == nil {
		t.Fatal("placeholder token accepted")
	}
}
