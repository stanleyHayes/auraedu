package webhooks

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/google/uuid"
	"github.com/twilio/twilio-go/client"
)

var (
	ErrInvalidTwilioSignature = errors.New("notifications: invalid Twilio webhook signature")
	ErrInvalidTwilioPayload   = errors.New("notifications: invalid Twilio webhook payload")
	twilioAccountSID          = regexp.MustCompile(`^AC[0-9a-fA-F]{32}$`)
	twilioMessageSID          = regexp.MustCompile(`^(SM|MM)[0-9a-fA-F]{32}$`)
	twilioE164                = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
)

// TwilioVerifier authenticates status callbacks using Twilio's official
// request validator and returns only the fields AuraEDU needs to correlate a
// delivery. The raw phone number and provider payload are never persisted.
type TwilioVerifier struct {
	accountSID string
	callback   string
	validator  client.RequestValidator
	now        func() time.Time
}

func NewTwilioVerifier(accountSID, authToken, callbackURL string, allowInsecure bool) (*TwilioVerifier, error) {
	accountSID = strings.TrimSpace(accountSID)
	authToken = strings.TrimSpace(authToken)
	callback, err := url.Parse(strings.TrimSpace(callbackURL))
	validCallback := err == nil && (callback.Scheme == "https" || (allowInsecure && callback.Scheme == "http")) && callback.Hostname() != "" &&
		callback.User == nil && callback.RawQuery == "" && callback.Fragment == "" &&
		callback.Path == "/api/v1/webhooks/twilio"
	if !validCallback {
		return nil, errors.New("notifications: Twilio webhook must use the public credential-free HTTPS callback URL")
	}
	host := strings.ToLower(callback.Hostname())
	if !allowInsecure && (callback.Port() != "" || host == "localhost" || host == "127.0.0.1" || strings.HasSuffix(host, ".example")) {
		return nil, errors.New("notifications: Twilio webhook must use the public credential-free HTTPS callback URL")
	}
	if !twilioAccountSID.MatchString(accountSID) || len(authToken) < 16 || len(authToken) > 128 {
		return nil, errors.New("notifications: valid Twilio webhook credentials are required")
	}
	return &TwilioVerifier{
		accountSID: accountSID,
		callback:   callback.String(),
		validator:  client.NewRequestValidator(authToken),
		now:        time.Now,
	}, nil
}

// Verify authenticates every form field against the exact configured callback
// URL, including AuraEDU's message_id query parameter. Unknown signed fields
// are accepted so provider payload additions do not break delivery tracking.
func (v *TwilioVerifier) Verify(messageID string, values url.Values, signature string) (ports.DeliveryFeedback, bool, error) {
	if v == nil || strings.TrimSpace(signature) == "" {
		return ports.DeliveryFeedback{}, false, ErrInvalidTwilioSignature
	}
	messageID = strings.TrimSpace(messageID)
	if _, err := uuid.Parse(messageID); err != nil {
		return ports.DeliveryFeedback{}, false, ErrInvalidTwilioPayload
	}
	callback, err := url.Parse(v.callback)
	if err != nil {
		return ports.DeliveryFeedback{}, false, ErrInvalidTwilioPayload
	}
	query := callback.Query()
	query.Set("message_id", messageID)
	callback.RawQuery = query.Encode()

	params := make(map[string]string, len(values))
	for key, entries := range values {
		if strings.TrimSpace(key) == "" || len(entries) != 1 {
			return ports.DeliveryFeedback{}, false, ErrInvalidTwilioSignature
		}
		params[key] = entries[0]
	}
	if !v.validator.Validate(callback.String(), params, strings.TrimSpace(signature)) {
		return ports.DeliveryFeedback{}, false, ErrInvalidTwilioSignature
	}
	if strings.TrimSpace(params["AccountSid"]) != v.accountSID {
		return ports.DeliveryFeedback{}, false, ErrInvalidTwilioPayload
	}
	providerMessageID := strings.TrimSpace(params["MessageSid"])
	if !twilioMessageSID.MatchString(providerMessageID) {
		return ports.DeliveryFeedback{}, false, ErrInvalidTwilioPayload
	}
	statusName := strings.ToLower(strings.TrimSpace(params["MessageStatus"]))
	status, relevant := twilioDeliveryStatus(statusName)
	if !relevant {
		return ports.DeliveryFeedback{}, false, nil
	}
	address := normalizeTwilioAddress(params["To"])
	if !twilioE164.MatchString(address) {
		return ports.DeliveryFeedback{}, false, ErrInvalidTwilioPayload
	}
	now := v.now().UTC()
	if now.IsZero() {
		return ports.DeliveryFeedback{}, false, ErrInvalidTwilioPayload
	}
	return ports.DeliveryFeedback{
		ID:                fmt.Sprintf("twilio:%s:%s", providerMessageID, statusName),
		Provider:          "twilio",
		ProviderMessageID: providerMessageID,
		MessageID:         messageID,
		EventType:         "twilio.message." + statusName,
		Status:            status,
		AddressHash:       fmt.Sprintf("%x", sha256.Sum256([]byte(address))),
		OccurredAt:        now,
	}, true, nil
}

func twilioDeliveryStatus(status string) (string, bool) {
	switch status {
	case "accepted", "queued", "sending", "sent":
		return string(domain.DeliveryStatusAccepted), true
	case "delivered", "read":
		return string(domain.DeliveryStatusDelivered), true
	case "failed", "undelivered", "canceled":
		return string(domain.DeliveryStatusFailed), true
	default:
		return "", false
	}
}

func normalizeTwilioAddress(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= len("whatsapp:") && strings.EqualFold(value[:len("whatsapp:")], "whatsapp:") {
		value = strings.TrimSpace(value[len("whatsapp:"):])
	}
	return value
}
