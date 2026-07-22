package webhooks

import (
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- Twilio's webhook protocol requires HMAC-SHA1.
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

var testTwilioWebhookAccount = "AC" + strings.Repeat("0", 32)

const (
	testTwilioWebhookToken = "0123456789abcdef0123456789abcdef"
	testTwilioWebhookSID   = "SM0123456789abcdef0123456789abcdef"
	testTwilioCallback     = "https://api.auraedu.com/api/v1/webhooks/twilio"
	testTwilioMessageID    = "77f7b178-6312-4fe7-8120-88393bf80b49"
)

func TestTwilioVerifierAuthenticatesAndMinimizesDeliveryEvent(t *testing.T) {
	verifier := testTwilioVerifier(t)
	values := url.Values{
		"AccountSid":    {testTwilioWebhookAccount},
		"MessageSid":    {testTwilioWebhookSID},
		"MessageStatus": {"delivered"},
		"To":            {"whatsapp:+233200000004"},
		"FutureField":   {"provider additions remain authenticated"},
	}
	signature := twilioTestSignature(values)
	feedback, relevant, err := verifier.Verify(testTwilioMessageID, values, signature)
	if err != nil || !relevant {
		t.Fatalf("feedback relevant=%v err=%v", relevant, err)
	}
	expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte("+233200000004")))
	if feedback.ID != "twilio:"+testTwilioWebhookSID+":delivered" || feedback.Provider != "twilio" ||
		feedback.ProviderMessageID != testTwilioWebhookSID || feedback.MessageID != testTwilioMessageID ||
		feedback.EventType != "twilio.message.delivered" || feedback.Status != "delivered" ||
		feedback.AddressHash != expectedHash || !feedback.OccurredAt.Equal(time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("feedback=%+v", feedback)
	}
	second, relevant, err := verifier.Verify(testTwilioMessageID, values, signature)
	if err != nil || !relevant || second.ID != feedback.ID {
		t.Fatalf("replay feedback=%+v relevant=%v err=%v", second, relevant, err)
	}
}

func TestTwilioVerifierRejectsTamperingAndDuplicateFields(t *testing.T) {
	verifier := testTwilioVerifier(t)
	values := url.Values{
		"AccountSid":    {testTwilioWebhookAccount},
		"MessageSid":    {testTwilioWebhookSID},
		"MessageStatus": {"delivered"},
		"To":            {"+233200000004"},
	}
	signature := twilioTestSignature(values)
	values.Set("To", "+233200000099")
	if _, _, err := verifier.Verify(testTwilioMessageID, values, signature); !errors.Is(err, ErrInvalidTwilioSignature) {
		t.Fatalf("tampered signature error=%v", err)
	}
	values["To"] = []string{"+233200000004", "+233200000099"}
	if _, _, err := verifier.Verify(testTwilioMessageID, values, signature); !errors.Is(err, ErrInvalidTwilioSignature) {
		t.Fatalf("duplicate field error=%v", err)
	}
}

func TestTwilioVerifierMapsWhatsAppReadAndIgnoresSignedUnknownStatus(t *testing.T) {
	verifier := testTwilioVerifier(t)
	values := url.Values{
		"AccountSid":    {testTwilioWebhookAccount},
		"MessageSid":    {testTwilioWebhookSID},
		"MessageStatus": {"read"},
		"To":            {"whatsapp:+233200000004"},
	}
	signature := twilioTestSignature(values)
	feedback, relevant, err := verifier.Verify(testTwilioMessageID, values, signature)
	if err != nil || !relevant || feedback.Status != "delivered" {
		t.Fatalf("read feedback=%+v relevant=%v err=%v", feedback, relevant, err)
	}
	values.Set("MessageStatus", "future-status")
	signature = twilioTestSignature(values)
	if _, relevant, err = verifier.Verify(testTwilioMessageID, values, signature); err != nil || relevant {
		t.Fatalf("unknown status relevant=%v err=%v", relevant, err)
	}
}

func TestTwilioVerifierRejectsMalformedCallbackWithoutPanicking(t *testing.T) {
	if _, err := NewTwilioVerifier(testTwilioWebhookAccount, testTwilioWebhookToken, "%invalid", false); err == nil {
		t.Fatal("malformed callback URL accepted")
	}
}

func testTwilioVerifier(t *testing.T) *TwilioVerifier {
	t.Helper()
	verifier, err := NewTwilioVerifier(testTwilioWebhookAccount, testTwilioWebhookToken, testTwilioCallback, false)
	if err != nil {
		t.Fatal(err)
	}
	verifier.now = func() time.Time { return time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC) }
	return verifier
}

func twilioTestSignature(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	payload := testTwilioCallback + "?message_id=" + testTwilioMessageID
	for _, key := range keys {
		payload += key + values.Get(key)
	}
	digest := hmac.New(sha1.New, []byte(testTwilioWebhookToken))
	_, _ = digest.Write([]byte(payload))
	return base64.StdEncoding.EncodeToString(digest.Sum(nil))
}
