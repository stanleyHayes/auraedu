package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/google/uuid"
)

const (
	defaultTwilioAPIBase = "https://api.twilio.com"
	maxTwilioResponse    = 64 << 10
	maxTwilioBodyRunes   = 1600
)

var (
	twilioAccountSIDPattern  = regexp.MustCompile(`^AC[0-9a-fA-F]{32}$`)
	twilioMessageSIDPattern  = regexp.MustCompile(`^(SM|MM)[0-9a-fA-F]{32}$`)
	twilioServiceSIDPattern  = regexp.MustCompile(`^MG[0-9a-fA-F]{32}$`)
	twilioAlphaSenderPattern = regexp.MustCompile(`^[A-Za-z0-9 ]{1,11}$`)
	e164Pattern              = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
)

// TwilioConfig configures the shared Programmable Messaging API used for SMS
// and WhatsApp. APIBase is configurable only to support hermetic development
// and tests; production construction requires a trusted Twilio HTTPS host.
type TwilioConfig struct {
	AccountSID         string
	AuthToken          string
	SMSFrom            string
	MessagingServiceID string
	WhatsAppFrom       string
	APIBase            string
	StatusCallbackURL  string
	AllowInsecure      bool
}

// TwilioNotifier delivers one channel through Twilio's Messages resource.
type TwilioNotifier struct {
	channel domain.NotificationChannel
	config  TwilioConfig
	client  *http.Client
	url     string
}

// NewTwilioNotifier creates an SMS or WhatsApp adapter. Channel-specific
// senders are validated before a worker can accept traffic.
func NewTwilioNotifier(
	config TwilioConfig,
	channel domain.NotificationChannel,
	client *http.Client,
) (*TwilioNotifier, error) {
	config, endpoint, err := normalizeTwilioConfig(config)
	if err != nil {
		return nil, err
	}
	if err := validateTwilioSender(config, channel); err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &TwilioNotifier{channel: channel, config: config, client: client, url: endpoint}, nil
}

func normalizeTwilioConfig(config TwilioConfig) (TwilioConfig, string, error) {
	config.AccountSID = strings.TrimSpace(config.AccountSID)
	config.AuthToken = strings.TrimSpace(config.AuthToken)
	config.SMSFrom = strings.TrimSpace(config.SMSFrom)
	config.MessagingServiceID = strings.TrimSpace(config.MessagingServiceID)
	config.WhatsAppFrom = stripWhatsAppPrefix(config.WhatsAppFrom)
	config.APIBase = strings.TrimRight(strings.TrimSpace(config.APIBase), "/")
	config.StatusCallbackURL = strings.TrimSpace(config.StatusCallbackURL)
	if config.APIBase == "" {
		config.APIBase = defaultTwilioAPIBase
	}
	if !twilioAccountSIDPattern.MatchString(config.AccountSID) || len(config.AuthToken) < 16 || len(config.AuthToken) > 128 {
		return config, "", errors.New("notifications: valid Twilio account credentials are required")
	}
	endpoint, err := twilioAPIEndpoint(config)
	if err != nil {
		return config, "", err
	}
	if err := validateTwilioCallback(config); err != nil {
		return config, "", err
	}
	return config, endpoint, nil
}

func twilioAPIEndpoint(config TwilioConfig) (string, error) {
	parsed, err := url.Parse(config.APIBase)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", errors.New("notifications: Twilio API base must be a credential-free origin")
	}
	if !config.AllowInsecure && (parsed.Scheme != "https" || !trustedTwilioHost(parsed.Hostname())) {
		return "", errors.New("notifications: Twilio API base must use a trusted HTTPS host")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("notifications: Twilio API base must use HTTP or HTTPS")
	}
	return fmt.Sprintf(
		"%s/2010-04-01/Accounts/%s/Messages.json",
		config.APIBase,
		url.PathEscape(config.AccountSID),
	), nil
}

func validateTwilioCallback(config TwilioConfig) error {
	callback, err := url.Parse(config.StatusCallbackURL)
	validCallback := err == nil && callback.Hostname() != "" && callback.User == nil && callback.RawQuery == "" &&
		callback.Fragment == "" && callback.Path == "/api/v1/webhooks/twilio"
	if !validCallback {
		return errors.New("notifications: Twilio status callback must be the credential-free AuraEDU webhook URL")
	}
	callbackHost := strings.ToLower(callback.Hostname())
	if !config.AllowInsecure && (callback.Scheme != "https" || callback.Port() != "" || callbackHost == "localhost" ||
		callbackHost == "127.0.0.1" || strings.HasSuffix(callbackHost, ".example")) {
		return errors.New("notifications: production Twilio status callback must use a public HTTPS origin")
	}
	if config.AllowInsecure && callback.Scheme != "http" && callback.Scheme != "https" {
		return errors.New("notifications: Twilio status callback must use HTTP or HTTPS")
	}
	return nil
}

func validateTwilioSender(config TwilioConfig, channel domain.NotificationChannel) error {
	switch channel {
	case domain.ChannelSMS:
		if config.MessagingServiceID != "" && !twilioServiceSIDPattern.MatchString(config.MessagingServiceID) {
			return errors.New("notifications: valid Twilio messaging service SID is required")
		}
		validFrom := e164Pattern.MatchString(config.SMSFrom) ||
			(strings.TrimSpace(config.SMSFrom) != "" && twilioAlphaSenderPattern.MatchString(config.SMSFrom))
		if config.MessagingServiceID == "" && !validFrom {
			return errors.New("notifications: Twilio SMS sender or messaging service is required")
		}
	case domain.ChannelWhatsApp:
		if !e164Pattern.MatchString(config.WhatsAppFrom) {
			return errors.New("notifications: valid Twilio WhatsApp sender is required")
		}
	default:
		return errors.New("notifications: Twilio notifier supports only sms or whatsapp")
	}
	return nil
}

// Send submits one bounded text message. Twilio's accepted response is
// validated before AuraEDU records the delivery attempt as sent.
func (n *TwilioNotifier) Send(ctx context.Context, message domain.Message) error {
	_, err := n.SendWithReceipt(ctx, message)
	return err
}

// SendWithReceipt submits a Twilio message and returns its stable SID for
// signed delivery-status correlation.
func (n *TwilioNotifier) SendWithReceipt(ctx context.Context, message domain.Message) (receipt ports.ProviderReceipt, returnErr error) {
	if message.Channel != string(n.channel) {
		return receipt, fmt.Errorf("notifications: Twilio adapter is configured for %s", n.channel)
	}
	body := strings.TrimSpace(message.Body)
	if body == "" || utf8.RuneCountInString(body) > maxTwilioBodyRunes {
		return receipt, errors.New("notifications: Twilio message body must contain 1 to 1600 characters")
	}
	recipient := deliveryPhone(message)
	if !e164Pattern.MatchString(recipient) {
		return receipt, errors.New("notifications: valid E.164 delivery_address is required")
	}
	values, err := n.messageForm(message.ID, recipient, body)
	if err != nil {
		return receipt, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, strings.NewReader(values.Encode()))
	if err != nil {
		return receipt, fmt.Errorf("notifications: create Twilio request: %w", err)
	}
	req.SetBasicAuth(n.config.AccountSID, n.config.AuthToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "AuraEDU-Notification/1.0")
	response, err := n.client.Do(req)
	if err != nil {
		return receipt, fmt.Errorf("notifications: Twilio request: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, response.Body.Close())
	}()
	encoded, err := io.ReadAll(io.LimitReader(response.Body, maxTwilioResponse+1))
	if err != nil {
		return receipt, fmt.Errorf("notifications: read Twilio response: %w", err)
	}
	if len(encoded) > maxTwilioResponse {
		return receipt, errors.New("notifications: Twilio response exceeded the size limit")
	}
	return validateTwilioResponse(response.StatusCode, encoded)
}

func (n *TwilioNotifier) messageForm(messageID, recipient, body string) (url.Values, error) {
	if _, err := uuid.Parse(messageID); err != nil {
		return nil, errors.New("notifications: valid AuraEDU message ID is required for Twilio delivery")
	}
	callback, err := url.Parse(n.config.StatusCallbackURL)
	if err != nil {
		return nil, errors.New("notifications: invalid Twilio status callback configuration")
	}
	query := callback.Query()
	query.Set("message_id", messageID)
	callback.RawQuery = query.Encode()
	values := url.Values{"Body": {body}, "StatusCallback": {callback.String()}}
	switch n.channel {
	case domain.ChannelSMS:
		values.Set("To", recipient)
		if n.config.MessagingServiceID != "" {
			values.Set("MessagingServiceSid", n.config.MessagingServiceID)
		} else {
			values.Set("From", n.config.SMSFrom)
		}
	case domain.ChannelWhatsApp:
		values.Set("To", "whatsapp:"+recipient)
		values.Set("From", "whatsapp:"+n.config.WhatsAppFrom)
	default:
		return nil, errors.New("notifications: unsupported Twilio channel")
	}
	return values, nil
}

type twilioReceipt struct {
	SID       string `json:"sid"`
	Status    string `json:"status"`
	ErrorCode *int   `json:"error_code"`
	Code      *int   `json:"code"`
}

func validateTwilioResponse(statusCode int, encoded []byte) (ports.ProviderReceipt, error) {
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return ports.ProviderReceipt{}, twilioHTTPError(statusCode, encoded)
	}
	var result twilioReceipt
	if err := json.Unmarshal(encoded, &result); err != nil {
		return ports.ProviderReceipt{}, fmt.Errorf("notifications: decode Twilio response: %w", err)
	}
	if !twilioMessageSIDPattern.MatchString(result.SID) || !validTwilioAcceptedStatus(result.Status) || result.ErrorCode != nil {
		return ports.ProviderReceipt{}, errors.New("notifications: Twilio returned an invalid delivery receipt")
	}
	return ports.ProviderReceipt{Provider: "twilio", MessageID: result.SID}, nil
}

func validTwilioAcceptedStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "accepted", "queued", "sending", "sent":
		return true
	default:
		return false
	}
}

func twilioHTTPError(statusCode int, encoded []byte) error {
	var result twilioReceipt
	if json.Unmarshal(encoded, &result) != nil {
		return fmt.Errorf("notifications: Twilio returned HTTP %d", statusCode)
	}
	code := result.ErrorCode
	if code == nil {
		code = result.Code
	}
	if code == nil {
		return fmt.Errorf("notifications: Twilio returned HTTP %d", statusCode)
	}
	return fmt.Errorf("notifications: Twilio returned HTTP %d (code %d)", statusCode, *code)
}

func deliveryPhone(message domain.Message) string {
	if value, ok := message.Metadata["delivery_address"].(string); ok {
		return stripWhatsAppPrefix(value)
	}
	return stripWhatsAppPrefix(message.RecipientID)
}

func stripWhatsAppPrefix(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= len("whatsapp:") && strings.EqualFold(value[:len("whatsapp:")], "whatsapp:") {
		return strings.TrimSpace(value[len("whatsapp:"):])
	}
	return value
}

func trustedTwilioHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "api.twilio.com" || strings.HasSuffix(host, ".twilio.com")
}

var (
	_ ports.Notifier        = (*TwilioNotifier)(nil)
	_ ports.ReceiptNotifier = (*TwilioNotifier)(nil)
)
