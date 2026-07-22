package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func testConfig(baseURL string) Config {
	return Config{
		Name: "auraedu-staging-messaging-providers", Environment: "staging", BaseURL: baseURL,
		Timeout: duration{2 * time.Second}, DeliveryTimeout: duration{10 * time.Second}, PollInterval: duration{time.Millisecond},
		RunID: "release-twilio-test", GitSHA: "abcdef1234567890", Tenant: "school-a",
		Token: "runtime-bearer-token-123456", RecipientID: "0198f0db-7d3d-7000-8000-000000000001",
		SMSNumber: "+233200000001", WhatsAppNumber: "+233200000002",
	}
}

func TestRunProvesBothChannelsWithoutRetainingSensitiveValues(t *testing.T) {
	const smsID = "0198f0db-7d3d-7000-8000-000000000002"
	const whatsappID = "0198f0db-7d3d-7000-8000-000000000003"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var mu sync.Mutex
	fetches := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer runtime-bearer-token-123456" || request.Header.Get("X-Tenant-Code") != "school-a" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPost && request.URL.Path == "/api/v1/messages" {
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode request: %v", err)
				writer.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			channel, ok := body["channel"].(string)
			if !ok {
				t.Error("request channel is missing")
				writer.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			id := smsID
			if channel == "whatsapp" {
				id = whatsappID
			}
			writer.WriteHeader(http.StatusCreated)
			if _, err := writer.Write([]byte(`{"id":"` + id + `","channel":"` + channel + `","status":"pending"}`)); err != nil {
				t.Errorf("write create response: %v", err)
			}
			return
		}
		id := smsID
		channel := "sms"
		if strings.Contains(request.URL.Path, whatsappID) {
			id = whatsappID
			channel = "whatsapp"
		}
		status := "accepted"
		if request.Method == http.MethodGet {
			mu.Lock()
			fetches[id]++
			if fetches[id] >= 1 {
				status = "delivered"
			}
			mu.Unlock()
		}
		if _, err := writer.Write([]byte(`{"id":"` + id + `","channel":"` + channel + `","status":"sent","sent_at":"` + now + `","provider":"twilio","delivery_status":"` + status + `","delivery_status_at":"` + now + `"}`)); err != nil {
			t.Errorf("write message response: %v", err)
		}
	}))
	defer server.Close()
	evidence, err := run(context.Background(), testConfig(server.URL))
	if err != nil || !evidence.AllPassed || len(evidence.Checks) != 6 {
		t.Fatalf("evidence=%+v err=%v", evidence, err)
	}
	encoded, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	for _, sensitive := range []string{"runtime-bearer-token-123456", "school-a", "+233200000001", "+233200000002", smsID, whatsappID} {
		if strings.Contains(string(encoded), sensitive) {
			t.Fatalf("evidence leaked sensitive value %q", sensitive)
		}
	}
}

func TestExecutionRejectsPlaceholderTargetsAndMalformedNumbers(t *testing.T) {
	cfg := testConfig("https://staging-api.auraedu.com")
	if err := validateExecution(cfg); err != nil {
		t.Fatal(err)
	}
	cfg.BaseURL = "https://staging.auraedu.example"
	if err := validateExecution(cfg); err == nil {
		t.Fatal("placeholder target accepted")
	}
	cfg.BaseURL = "https://staging-api.auraedu.com"
	cfg.WhatsAppNumber = "0200000002"
	if err := validateExecution(cfg); err == nil {
		t.Fatal("non-E.164 number accepted")
	}
}

func TestEvidenceOutputIsExclusiveAndPrivate(t *testing.T) {
	path := t.TempDir() + "/twilio.json"
	if err := writeEvidence(path, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if err := writeEvidence(path, []byte("overwrite\n")); err == nil {
		t.Fatal("evidence output allowed overwrite")
	}
}
