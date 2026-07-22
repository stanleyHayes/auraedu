// Package notifier provides notification channel adapters for the notification service.
package notifier

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
)

// MockNotifier is a deterministic notifier for tests and local development.
// It succeeds unless the message body contains the word "fail" (case-insensitive).
type MockNotifier struct {
	channel string
}

// NewMockNotifier creates a mock notifier for the given channel.
func NewMockNotifier(channel string) *MockNotifier {
	return &MockNotifier{channel: channel}
}

// Send attempts to deliver the message. It fails when the body contains "fail".
func (n *MockNotifier) Send(ctx context.Context, msg domain.Message) error {
	_ = ctx
	if strings.Contains(strings.ToLower(msg.Body), "fail") {
		return fmt.Errorf("mock %s notifier: forced failure", n.channel)
	}
	return nil
}

// Registry returns a map of mock notifiers for all supported channels.
func Registry() map[string]ports.Notifier {
	return map[string]ports.Notifier{
		"email":    NewMockNotifier("email"),
		"sms":      NewMockNotifier("sms"),
		"whatsapp": NewMockNotifier("whatsapp"),
		"in_app":   NewMockNotifier("in_app"),
		"push":     NewMockNotifier("push"),
	}
}

// RegistryFromEnv returns production adapters. Production refuses to start
// with mock delivery, preventing false "sent" records that never left AuraEDU.
func RegistryFromEnv() (map[string]ports.Notifier, error) {
	return RegistryFromEnvWithPush(nil)
}

// RegistryFromEnvWithPush builds the provider registry and enables real Expo
// delivery when a device repository is supplied by the server or worker.
func RegistryFromEnvWithPush(devices ports.DeviceTokenRepository) (map[string]ports.Notifier, error) {
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("NOTIFICATION_PROVIDER")))
	if provider == "" && os.Getenv("SMTP_HOST") != "" {
		provider = "smtp"
	}
	if provider == "" {
		provider = "mock"
	}
	if provider == "mock" {
		if environment == "production" {
			return nil, errors.New("notifications: mock provider is forbidden in production")
		}
		return Registry(), nil
	}
	var email ports.Notifier
	var err error
	switch provider {
	case "resend":
		email, err = resendNotifierFromEnv(environment)
	case "smtp":
		email, err = smtpNotifierFromEnv(environment)
	default:
		return nil, fmt.Errorf("notifications: unsupported provider %q", provider)
	}
	if err != nil {
		return nil, err
	}
	registry := map[string]ports.Notifier{
		"email": email, "sms": UnconfiguredNotifier("sms"),
		"whatsapp": UnconfiguredNotifier("whatsapp"), "in_app": InboxNotifier{}, "push": UnconfiguredNotifier("push"),
	}
	if err := configureTwilioFromEnv(registry, environment); err != nil {
		return nil, err
	}
	if err := configureExpoPush(registry, devices, environment); err != nil {
		return nil, err
	}
	return registry, nil
}

func resendNotifierFromEnv(environment string) (ports.Notifier, error) {
	return NewResendNotifier(ResendConfig{
		APIKey: firstNonEmpty(os.Getenv("RESEND_API_KEY"), os.Getenv("SMTP_PASSWORD")),
		From:   firstNonEmpty(os.Getenv("RESEND_FROM_EMAIL"), os.Getenv("SMTP_FROM_EMAIL")),
		FromName: firstNonEmpty(
			os.Getenv("RESEND_FROM_NAME"), os.Getenv("SMTP_FROM_NAME"), "AuraEDU",
		),
		APIBase:       envOr("RESEND_API_BASE", defaultResendAPIBase),
		AllowInsecure: environment != "production",
	}, nil)
}

func smtpNotifierFromEnv(environment string) (ports.Notifier, error) {
	port, err := strconv.Atoi(envOr("SMTP_PORT", "587"))
	if err != nil || port < 1 || port > 65535 {
		return nil, errors.New("notifications: SMTP_PORT is invalid")
	}
	cfg := SMTPConfig{
		Host: os.Getenv("SMTP_HOST"), Port: port, Username: os.Getenv("SMTP_USERNAME"), Password: os.Getenv("SMTP_PASSWORD"),
		From: os.Getenv("SMTP_FROM_EMAIL"), FromName: envOr("SMTP_FROM_NAME", "AuraEDU"),
		AllowInsecure: strings.EqualFold(os.Getenv("SMTP_ALLOW_INSECURE"), "true"),
	}
	if cfg.Host == "" || cfg.From == "" {
		return nil, errors.New("notifications: SMTP_HOST and SMTP_FROM_EMAIL are required")
	}
	if environment == "production" && cfg.AllowInsecure {
		return nil, errors.New("notifications: SMTP_ALLOW_INSECURE is forbidden in production")
	}
	return NewSMTPNotifier(cfg), nil
}

func configureExpoPush(registry map[string]ports.Notifier, devices ports.DeviceTokenRepository, environment string) error {
	if devices == nil {
		return nil
	}
	push, err := NewExpoPushNotifier(devices, nil, ExpoPushConfig{
		URL: envOr("EXPO_PUSH_URL", defaultExpoPushURL), AccessToken: os.Getenv("EXPO_ACCESS_TOKEN"),
		AllowInsecure: environment != "production",
	})
	if err != nil {
		return err
	}
	registry["push"] = push
	return nil
}

func configureTwilioFromEnv(registry map[string]ports.Notifier, environment string) error {
	config := TwilioConfig{
		AccountSID: os.Getenv("TWILIO_ACCOUNT_SID"), AuthToken: os.Getenv("TWILIO_AUTH_TOKEN"),
		SMSFrom: os.Getenv("TWILIO_SMS_FROM"), MessagingServiceID: os.Getenv("TWILIO_MESSAGING_SERVICE_SID"),
		WhatsAppFrom: os.Getenv("TWILIO_WHATSAPP_FROM"), APIBase: envOr("TWILIO_API_BASE", defaultTwilioAPIBase),
		StatusCallbackURL: os.Getenv("TWILIO_STATUS_CALLBACK_URL"),
		AllowInsecure:     environment != "production",
	}
	configured := config.AccountSID != "" || config.AuthToken != "" || config.SMSFrom != "" ||
		config.MessagingServiceID != "" || config.WhatsAppFrom != ""
	if !configured {
		return nil
	}
	if config.SMSFrom == "" && config.MessagingServiceID == "" && config.WhatsAppFrom == "" {
		return errors.New("notifications: a Twilio SMS or WhatsApp sender is required when credentials are configured")
	}
	if config.SMSFrom != "" || config.MessagingServiceID != "" {
		sms, err := NewTwilioNotifier(config, domain.ChannelSMS, nil)
		if err != nil {
			return err
		}
		registry[string(domain.ChannelSMS)] = sms
	}
	if config.WhatsAppFrom != "" {
		whatsapp, err := NewTwilioNotifier(config, domain.ChannelWhatsApp, nil)
		if err != nil {
			return err
		}
		registry[string(domain.ChannelWhatsApp)] = whatsapp
	}
	return nil
}

type SMTPConfig struct {
	Host                               string
	Port                               int
	Username, Password, From, FromName string
	AllowInsecure                      bool
}
type SMTPNotifier struct {
	config  SMTPConfig
	dialer  net.Dialer
	rootCAs *x509.CertPool
}

func NewSMTPNotifier(config SMTPConfig) *SMTPNotifier {
	return &SMTPNotifier{config: config, dialer: net.Dialer{Timeout: 10 * time.Second}}
}

func (n *SMTPNotifier) Send(ctx context.Context, msg domain.Message) (returnErr error) {
	to, ok := msg.Metadata["delivery_address"].(string)
	if !ok {
		to = ""
	}
	if to == "" && strings.Contains(msg.RecipientID, "@") {
		to = msg.RecipientID
	}
	if strings.ContainsAny(to+n.config.From, "\r\n") || !strings.Contains(to, "@") {
		return errors.New("notifications: valid email delivery_address is required")
	}
	address := net.JoinHostPort(n.config.Host, strconv.Itoa(n.config.Port))
	tlsConfig := &tls.Config{ServerName: n.config.Host, MinVersion: tls.VersionTLS12, RootCAs: n.rootCAs}
	client, err := n.connect(ctx, address, tlsConfig)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			returnErr = errors.Join(returnErr, client.Close())
		}
	}()
	if n.config.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", n.config.Username, n.config.Password, n.config.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(n.config.From); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	subject := strings.NewReplacer("\r", " ", "\n", " ").Replace(msg.Subject)
	fromName := strings.NewReplacer("\r", " ", "\n", " ").Replace(n.config.FromName)
	payload := fmt.Sprintf(
		"From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: <%s@auraedu>\r\n"+
			"MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		fromName, n.config.From, to, subject, msg.ID, msg.Body,
	)
	if _, err := io.WriteString(w, payload); err != nil {
		return errors.Join(fmt.Errorf("smtp write: %w", err), w.Close())
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}
	closed = true
	return nil
}

func (n *SMTPNotifier) connect(ctx context.Context, address string, tlsConfig *tls.Config) (*smtp.Client, error) {
	conn, err := n.dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("smtp dial: %w", err)
	}
	deadline := time.Now().Add(n.dialer.Timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, errors.Join(fmt.Errorf("smtp deadline: %w", err), conn.Close())
	}
	if n.config.Port == 465 {
		return n.connectImplicitTLS(ctx, conn, tlsConfig)
	}
	client, err := smtp.NewClient(conn, n.config.Host)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("smtp client: %w", err), conn.Close())
	}
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(tlsConfig); err != nil {
			return nil, errors.Join(fmt.Errorf("smtp STARTTLS: %w", err), client.Close())
		}
	} else if !n.config.AllowInsecure {
		return nil, errors.Join(errors.New("smtp server does not offer STARTTLS"), client.Close())
	}
	return client, nil
}

func (n *SMTPNotifier) connectImplicitTLS(
	ctx context.Context,
	conn net.Conn,
	tlsConfig *tls.Config,
) (*smtp.Client, error) {
	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("smtp TLS handshake: %w", err), conn.Close())
	}
	client, err := smtp.NewClient(tlsConn, n.config.Host)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("smtp client: %w", err), tlsConn.Close())
	}
	return client, nil
}

type UnconfiguredNotifier string

func (n UnconfiguredNotifier) Send(context.Context, domain.Message) error {
	return fmt.Errorf("notifications: %s provider is not configured", string(n))
}

type InboxNotifier struct{}

func (InboxNotifier) Send(context.Context, domain.Message) error { return nil }

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

var _ ports.Notifier = (*MockNotifier)(nil)

// ErrNoNotifier is returned when a channel has no registered notifier.
var ErrNoNotifier = errors.New("notifications: no notifier configured for channel")
