package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/google/uuid"
)

const (
	defaultResendAPIBase = "https://api.resend.com"
	maxResendResponse    = 64 << 10
)

type ResendConfig struct {
	APIKey        string
	From          string
	FromName      string
	APIBase       string
	AllowInsecure bool
}

type ResendNotifier struct {
	config ResendConfig
	client *http.Client
}

func NewResendNotifier(config ResendConfig, client *http.Client) (*ResendNotifier, error) {
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.From = strings.TrimSpace(config.From)
	config.FromName = strings.TrimSpace(config.FromName)
	if config.APIBase == "" {
		config.APIBase = defaultResendAPIBase
	}
	parsed, err := url.Parse(strings.TrimRight(config.APIBase, "/"))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
		return nil, errors.New("notifications: Resend API base must be a credential-free origin")
	}
	if parsed.Scheme != "https" && (!config.AllowInsecure || parsed.Scheme != "http") {
		return nil, errors.New("notifications: Resend API base must use HTTPS")
	}
	if !config.AllowInsecure && !strings.EqualFold(parsed.Hostname(), "api.resend.com") {
		return nil, errors.New("notifications: production Resend API base must use api.resend.com")
	}
	if config.APIKey == "" || config.From == "" {
		return nil, errors.New("notifications: RESEND_API_KEY and RESEND_FROM_EMAIL are required")
	}
	if _, err := mail.ParseAddress(config.From); err != nil || strings.ContainsAny(config.From+config.FromName, "\r\n") {
		return nil, errors.New("notifications: valid Resend sender is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	config.APIBase = strings.TrimRight(config.APIBase, "/")
	return &ResendNotifier{config: config, client: client}, nil
}

func (n *ResendNotifier) Send(ctx context.Context, msg domain.Message) error {
	_, err := n.SendWithReceipt(ctx, msg)
	return err
}

func (n *ResendNotifier) SendWithReceipt(ctx context.Context, msg domain.Message) (receipt ports.ProviderReceipt, returnErr error) {
	to, ok := msg.Metadata["delivery_address"].(string)
	if !ok {
		to = ""
	}
	if to == "" && strings.Contains(msg.RecipientID, "@") {
		to = msg.RecipientID
	}
	to = strings.TrimSpace(strings.ToLower(to))
	parsedTo, err := mail.ParseAddress(to)
	if err != nil || parsedTo.Address != to || strings.ContainsAny(to, "\r\n") {
		return ports.ProviderReceipt{}, errors.New("notifications: valid email delivery_address is required")
	}
	from := n.config.From
	if n.config.FromName != "" {
		from = (&mail.Address{Name: n.config.FromName, Address: n.config.From}).String()
	}
	payload := struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
		Tags    []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"tags"`
	}{From: from, To: []string{to}, Subject: msg.Subject, Text: msg.Body}
	payload.Tags = append(payload.Tags, struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{Name: "aura_message", Value: msg.ID})
	body, err := json.Marshal(payload)
	if err != nil {
		return ports.ProviderReceipt{}, fmt.Errorf("notifications: encode Resend request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.config.APIBase+"/emails", bytes.NewReader(body))
	if err != nil {
		return ports.ProviderReceipt{}, fmt.Errorf("notifications: create Resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+n.config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "auraedu-notification-service/1.0")
	req.Header.Set("Idempotency-Key", msg.ID)
	response, err := n.client.Do(req)
	if err != nil {
		return ports.ProviderReceipt{}, fmt.Errorf("notifications: Resend request: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, response.Body.Close())
	}()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResendResponse+1))
	if err != nil {
		return ports.ProviderReceipt{}, fmt.Errorf("notifications: read Resend response: %w", err)
	}
	if len(responseBody) > maxResendResponse {
		return ports.ProviderReceipt{}, errors.New("notifications: Resend response exceeded the size limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ports.ProviderReceipt{}, fmt.Errorf("notifications: Resend returned HTTP %d", response.StatusCode)
	}
	var responseReceipt struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(responseBody, &responseReceipt); err != nil {
		return ports.ProviderReceipt{}, fmt.Errorf("notifications: decode Resend response: %w", err)
	}
	if _, err := uuid.Parse(responseReceipt.ID); err != nil {
		return ports.ProviderReceipt{}, errors.New("notifications: Resend returned an invalid delivery receipt")
	}
	return ports.ProviderReceipt{Provider: "resend", MessageID: responseReceipt.ID}, nil
}

var _ ports.ReceiptNotifier = (*ResendNotifier)(nil)
