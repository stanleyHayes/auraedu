package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	providerwebhooks "github.com/auraedu/notification-service/internal/adapters/webhooks"
	"github.com/auraedu/notification-service/internal/ports"
)

type captureTwilioVerifier struct {
	messageID string
	values    url.Values
	signature string
	err       error
}

func (v *captureTwilioVerifier) Verify(messageID string, values url.Values, signature string) (ports.DeliveryFeedback, bool, error) {
	v.messageID = messageID
	v.values = values
	v.signature = signature
	return ports.DeliveryFeedback{}, false, v.err
}

func TestTwilioWebhookParsesBoundedFormAndReturnsProviderSuccess(t *testing.T) {
	verifier := &captureTwilioVerifier{}
	handler := NewHandler(nil).WithTwilioWebhookVerifier(verifier)
	mux := http.NewServeMux()
	handler.Register(mux)
	body := "AccountSid=" + "AC" + strings.Repeat("0", 32) + "&MessageStatus=delivered&FutureField=signed"
	request := httptest.NewRequestWithContext(
		context.Background(), http.MethodPost,
		"/api/v1/webhooks/twilio?message_id=77f7b178-6312-4fe7-8120-88393bf80b49", strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	request.Header.Set("X-Twilio-Signature", "authenticated")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || verifier.messageID != "77f7b178-6312-4fe7-8120-88393bf80b49" ||
		verifier.values.Get("FutureField") != "signed" || verifier.signature != "authenticated" {
		t.Fatalf("status=%d verifier=%+v body=%s", response.Code, verifier, response.Body.String())
	}
}

func TestTwilioWebhookFailsClosedForInvalidSignatureAndMediaType(t *testing.T) {
	verifier := &captureTwilioVerifier{err: providerwebhooks.ErrInvalidTwilioSignature}
	handler := NewHandler(nil).WithTwilioWebhookVerifier(verifier)
	mux := http.NewServeMux()
	handler.Register(mux)
	request := httptest.NewRequestWithContext(
		context.Background(), http.MethodPost,
		"/api/v1/webhooks/twilio?message_id=77f7b178-6312-4fe7-8120-88393bf80b49", strings.NewReader("MessageStatus=delivered"),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("signature status=%d body=%s", response.Code, response.Body.String())
	}

	verifier.err = errors.New("must not be called")
	request = httptest.NewRequestWithContext(
		context.Background(), http.MethodPost,
		"/api/v1/webhooks/twilio?message_id=77f7b178-6312-4fe7-8120-88393bf80b49", strings.NewReader(`{"MessageStatus":"delivered"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("media type status=%d body=%s", response.Code, response.Body.String())
	}
}
