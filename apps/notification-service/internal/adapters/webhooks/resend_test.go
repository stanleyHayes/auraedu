package webhooks

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	svix "github.com/svix/svix-webhooks/go"
)

func TestResendVerifierAuthenticatesAndMinimizesDeliveryEvent(t *testing.T) {
	verifier, signer := testVerifier(t)
	messageID := uuid.NewString()
	providerID := uuid.NewString()
	payload := []byte(fmt.Sprintf(`{"type":"email.bounced","created_at":"2026-07-21T08:00:00Z","data":{"email_id":%q,"to":["Teacher@Example.com"],"subject":"private","tags":{"aura_message":%q}}}`, providerID, messageID))
	headers := signedHeaders(t, signer, "msg_delivery_1", payload)
	feedback, relevant, err := verifier.Verify(payload, headers)
	if err != nil || !relevant {
		t.Fatalf("feedback relevant=%v err=%v", relevant, err)
	}
	if feedback.ID != "msg_delivery_1" || feedback.Provider != "resend" || feedback.ProviderMessageID != providerID ||
		feedback.MessageID != messageID || feedback.Status != "bounced" || len(feedback.AddressHash) != 64 {
		t.Fatalf("feedback=%+v", feedback)
	}
}

func TestResendVerifierRejectsTamperingAndIgnoresTrackingEvents(t *testing.T) {
	verifier, signer := testVerifier(t)
	messageID := uuid.NewString()
	providerID := uuid.NewString()
	payload := []byte(fmt.Sprintf(`{"type":"email.opened","created_at":"2026-07-21T08:00:00Z","data":{"email_id":%q,"to":["teacher@example.com"],"tags":{"aura_message":%q}}}`, providerID, messageID))
	headers := signedHeaders(t, signer, "msg_tracking_1", payload)
	if _, relevant, err := verifier.Verify(payload, headers); err != nil || relevant {
		t.Fatalf("tracking event relevant=%v err=%v", relevant, err)
	}
	payload[1] = 'X'
	if _, _, err := verifier.Verify(payload, headers); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("tampered signature error=%v", err)
	}
}

func testVerifier(t *testing.T) (*ResendVerifier, *svix.Webhook) {
	t.Helper()
	secret := "whsec_" + base64.StdEncoding.EncodeToString([]byte("auraedu-resend-webhook-test-secret"))
	verifier, err := NewResendVerifier(secret)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := svix.NewWebhook(secret)
	if err != nil {
		t.Fatal(err)
	}
	return verifier, signer
}

func signedHeaders(t *testing.T, signer *svix.Webhook, id string, payload []byte) http.Header {
	t.Helper()
	now := time.Now().UTC()
	signature, err := signer.Sign(id, now, payload)
	if err != nil {
		t.Fatal(err)
	}
	headers := make(http.Header)
	headers.Set("svix-id", id)
	headers.Set("svix-timestamp", fmt.Sprintf("%d", now.Unix()))
	headers.Set("svix-signature", signature)
	return headers
}
