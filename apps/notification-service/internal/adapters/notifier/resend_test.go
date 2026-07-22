package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/google/uuid"
)

func TestResendDeliversCorrelatedIdempotentEmail(t *testing.T) {
	messageID := uuid.NewString()
	providerID := uuid.NewString()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/emails" {
			t.Errorf("request=%s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer re_test_key" || request.Header.Get("Idempotency-Key") != messageID {
			t.Errorf("missing provider authentication or idempotency headers")
		}
		if request.Header.Get("User-Agent") != "auraedu-notification-service/1.0" {
			t.Errorf("user-agent=%q", request.Header.Get("User-Agent"))
		}
		var payload struct {
			From    string   `json:"from"`
			To      []string `json:"to"`
			Subject string   `json:"subject"`
			Text    string   `json:"text"`
			Tags    []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"tags"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if payload.From != `"AuraEDU" <onboarding@resend.dev>` || len(payload.To) != 1 || payload.To[0] != "teacher@example.com" ||
			payload.Subject != "Welcome" || payload.Text != "Your account is ready" || len(payload.Tags) != 1 ||
			payload.Tags[0].Name != "aura_message" || payload.Tags[0].Value != messageID {
			t.Errorf("unexpected Resend payload: %+v", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(writer).Encode(map[string]string{"id": providerID})
	}))
	defer server.Close()

	notifier, err := NewResendNotifier(ResendConfig{
		APIKey: "re_test_key", From: "onboarding@resend.dev", FromName: "AuraEDU",
		APIBase: server.URL, AllowInsecure: true,
	}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := notifier.SendWithReceipt(context.Background(), domain.Message{
		ID: messageID, Subject: "Welcome", Body: "Your account is ready",
		Metadata: map[string]any{"delivery_address": "teacher@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Provider != "resend" || receipt.MessageID != providerID {
		t.Fatalf("receipt=%+v", receipt)
	}
}

func TestResendProviderFailureDoesNotLeakBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnprocessableEntity)
		if _, err := writer.Write([]byte(`{"message":"teacher@example.com is restricted","secret":"do-not-log"}`)); err != nil {
			t.Errorf("write provider response: %v", err)
		}
	}))
	defer server.Close()
	notifier, err := NewResendNotifier(ResendConfig{APIKey: "re_test_key", From: "onboarding@resend.dev", APIBase: server.URL, AllowInsecure: true}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	err = notifier.Send(context.Background(), domain.Message{ID: uuid.NewString(), Subject: "Welcome", Body: "Body", Metadata: map[string]any{"delivery_address": "teacher@example.com"}})
	if err == nil || err.Error() != "notifications: Resend returned HTTP 422" {
		t.Fatalf("safe provider error=%v", err)
	}
}

func TestResendProductionConfigurationRejectsUntrustedOrigin(t *testing.T) {
	for _, base := range []string{"http://api.resend.com", "https://resend.example", "https://user:secret@api.resend.com"} {
		if _, err := NewResendNotifier(ResendConfig{APIKey: "re_test_key", From: "onboarding@resend.dev", APIBase: base}, nil); err == nil {
			t.Fatalf("unsafe Resend base accepted: %s", base)
		}
	}
}

func TestRegistryUsesExistingSMTPPasswordAsResendKeyFallback(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("NOTIFICATION_PROVIDER", "resend")
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("SMTP_PASSWORD", "re_existing_key")
	t.Setenv("RESEND_FROM_EMAIL", "onboarding@resend.dev")
	registry, err := RegistryFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry["email"].(*ResendNotifier); !ok {
		t.Fatalf("email adapter=%T", registry["email"])
	}
}
