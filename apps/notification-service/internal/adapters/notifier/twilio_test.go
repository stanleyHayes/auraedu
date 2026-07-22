package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
)

var testTwilioAccount = "AC" + strings.Repeat("0", 32)

const (
	testTwilioToken   = "0123456789abcdef0123456789abcdef"
	testTwilioSID     = "SM0123456789abcdef0123456789abcdef"
	testAuraMessageID = "77f7b178-6312-4fe7-8120-88393bf80b49"
)

func TestTwilioSMSDeliversBoundedFormWithBasicAuthentication(t *testing.T) {
	received := make(chan url.Values, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/2010-04-01/Accounts/"+testTwilioAccount+"/Messages.json" {
			t.Errorf("path=%q", request.URL.Path)
		}
		username, password, ok := request.BasicAuth()
		if !ok || username != testTwilioAccount || password != testTwilioToken {
			t.Error("Twilio request did not use the configured basic credentials")
		}
		if err := request.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		received <- request.PostForm
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(writer).Encode(map[string]any{"sid": testTwilioSID, "status": "queued", "error_code": nil})
	}))
	defer server.Close()
	notifier, err := NewTwilioNotifier(TwilioConfig{
		AccountSID: testTwilioAccount, AuthToken: testTwilioToken,
		MessagingServiceID: "MG0123456789abcdef0123456789abcdef",
		APIBase:            server.URL, StatusCallbackURL: server.URL + "/api/v1/webhooks/twilio", AllowInsecure: true,
	}, domain.ChannelSMS, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	message := domain.Message{ID: testAuraMessageID, Channel: "sms", RecipientID: "user-id", Body: "Application reminder",
		Metadata: map[string]any{"delivery_address": "+233200000001"}}
	receipt, err := notifier.SendWithReceipt(context.Background(), message)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Provider != "twilio" || receipt.MessageID != testTwilioSID {
		t.Fatalf("receipt=%+v", receipt)
	}
	form := <-received
	if form.Get("To") != "+233200000001" || form.Get("Body") != "Application reminder" ||
		form.Get("MessagingServiceSid") != "MG0123456789abcdef0123456789abcdef" || form.Get("From") != "" {
		t.Fatalf("unexpected Twilio form: %v", form)
	}
	if form.Get("StatusCallback") != server.URL+"/api/v1/webhooks/twilio?message_id="+testAuraMessageID {
		t.Fatalf("status callback=%q", form.Get("StatusCallback"))
	}
}

func TestTwilioWhatsAppPrefixesBothChannelAddresses(t *testing.T) {
	received := make(chan url.Values, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		received <- request.PostForm
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		if _, err := io.WriteString(writer, `{"sid":"`+testTwilioSID+`","status":"accepted","error_code":null}`); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()
	notifier, err := NewTwilioNotifier(TwilioConfig{
		AccountSID: testTwilioAccount, AuthToken: testTwilioToken,
		WhatsAppFrom: "whatsapp:+14155238886", APIBase: server.URL,
		StatusCallbackURL: server.URL + "/api/v1/webhooks/twilio", AllowInsecure: true,
	}, domain.ChannelWhatsApp, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err := notifier.Send(context.Background(), domain.Message{
		ID: testAuraMessageID, Channel: "whatsapp", RecipientID: "whatsapp:+233200000002", Body: "Your offer is ready",
	}); err != nil {
		t.Fatal(err)
	}
	form := <-received
	if form.Get("To") != "whatsapp:+233200000002" || form.Get("From") != "whatsapp:+14155238886" {
		t.Fatalf("unexpected channel addresses: %v", form)
	}
}

func TestTwilioRejectsInvalidRecipientBeforeNetwork(t *testing.T) {
	notifier, err := NewTwilioNotifier(TwilioConfig{
		AccountSID: testTwilioAccount, AuthToken: testTwilioToken,
		SMSFrom: "+12025550123", APIBase: "http://127.0.0.1:1",
		StatusCallbackURL: "http://127.0.0.1:1/api/v1/webhooks/twilio", AllowInsecure: true,
	}, domain.ChannelSMS, &http.Client{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	err = notifier.Send(context.Background(), domain.Message{ID: testAuraMessageID, Channel: "sms", RecipientID: "020 000 0001", Body: "Reminder"})
	if err == nil || !strings.Contains(err.Error(), "E.164") {
		t.Fatalf("invalid recipient error=%v", err)
	}
}

func TestTwilioProviderFailureDoesNotLeakResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		if _, err := io.WriteString(writer, `{"code":21608,"message":"recipient +233200000003 is not verified","more_info":"secret"}`); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()
	notifier, err := NewTwilioNotifier(TwilioConfig{
		AccountSID: testTwilioAccount, AuthToken: testTwilioToken,
		SMSFrom: "+12025550123", APIBase: server.URL,
		StatusCallbackURL: server.URL + "/api/v1/webhooks/twilio", AllowInsecure: true,
	}, domain.ChannelSMS, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	err = notifier.Send(context.Background(), domain.Message{ID: testAuraMessageID, Channel: "sms", RecipientID: "+233200000003", Body: "Reminder"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 400 (code 21608)") || strings.Contains(err.Error(), "+233") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("provider error was not safely bounded: %v", err)
	}
}

func TestTwilioRejectsFailedSuccessReceipt(t *testing.T) {
	encoded := []byte(`{"sid":"` + testTwilioSID + `","status":"failed","error_code":null}`)
	if _, err := validateTwilioResponse(http.StatusCreated, encoded); err == nil {
		t.Fatal("failed provider receipt must not be recorded as sent")
	}
}

func TestProductionTwilioConfigurationRequiresTrustedHTTPS(t *testing.T) {
	for _, base := range []string{"http://api.twilio.com", "https://twilio.example", "https://user:secret@api.twilio.com"} {
		_, err := NewTwilioNotifier(TwilioConfig{
			AccountSID: testTwilioAccount, AuthToken: testTwilioToken,
			SMSFrom: "+12025550123", APIBase: base,
		}, domain.ChannelSMS, nil)
		if err == nil {
			t.Fatalf("unsafe Twilio base accepted: %s", base)
		}
	}
}

func TestRegistryEnablesConfiguredTwilioChannels(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("NOTIFICATION_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.test")
	t.Setenv("SMTP_FROM_EMAIL", "notifications@example.test")
	t.Setenv("TWILIO_ACCOUNT_SID", testTwilioAccount)
	t.Setenv("TWILIO_AUTH_TOKEN", testTwilioToken)
	t.Setenv("TWILIO_SMS_FROM", "+12025550123")
	t.Setenv("TWILIO_WHATSAPP_FROM", "+14155238886")
	t.Setenv("TWILIO_STATUS_CALLBACK_URL", "http://localhost:18080/api/v1/webhooks/twilio")
	registry, err := RegistryFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry["sms"].(*TwilioNotifier); !ok {
		t.Fatalf("sms adapter=%T", registry["sms"])
	}
	if _, ok := registry["whatsapp"].(*TwilioNotifier); !ok {
		t.Fatalf("whatsapp adapter=%T", registry["whatsapp"])
	}
}

func TestRegistryRejectsPartialTwilioConfiguration(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("NOTIFICATION_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "smtp.example.test")
	t.Setenv("SMTP_FROM_EMAIL", "notifications@example.test")
	t.Setenv("TWILIO_ACCOUNT_SID", testTwilioAccount)
	t.Setenv("TWILIO_AUTH_TOKEN", testTwilioToken)
	if _, err := RegistryFromEnv(); err == nil || !strings.Contains(err.Error(), "sender is required") {
		t.Fatalf("partial Twilio configuration must fail closed, got %v", err)
	}
}
