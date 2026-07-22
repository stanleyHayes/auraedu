package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
)

const defaultExpoPushURL = "https://exp.host/--/api/v2/push/send"

type ExpoPushConfig struct {
	URL           string
	AccessToken   string
	AllowInsecure bool
}

type ExpoPushNotifier struct {
	devices ports.DeviceTokenRepository
	client  *http.Client
	config  ExpoPushConfig
}

func NewExpoPushNotifier(devices ports.DeviceTokenRepository, client *http.Client, config ExpoPushConfig) (*ExpoPushNotifier, error) {
	if devices == nil {
		return nil, errors.New("notifications: device token repository is required for push delivery")
	}
	if strings.TrimSpace(config.URL) == "" {
		config.URL = defaultExpoPushURL
	}
	parsed, err := url.Parse(config.URL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && !config.AllowInsecure) {
		return nil, errors.New("notifications: Expo push URL must be HTTPS")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &ExpoPushNotifier{devices: devices, client: client, config: config}, nil
}

type expoPushRequest struct {
	To       string         `json:"to"`
	Title    string         `json:"title"`
	Body     string         `json:"body"`
	Sound    string         `json:"sound"`
	Priority string         `json:"priority"`
	Data     map[string]any `json:"data,omitempty"`
}

type expoPushResponse struct {
	Data []struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Details struct {
			Error string `json:"error"`
		} `json:"details"`
	} `json:"data"`
}

func (n *ExpoPushNotifier) Send(ctx context.Context, message domain.Message) (returnErr error) {
	if message.Channel != string(domain.ChannelPush) {
		return errors.New("notifications: Expo notifier only supports push messages")
	}
	devices, err := n.devices.ListActive(ctx, message.TenantID, message.RecipientID)
	if err != nil {
		return fmt.Errorf("notifications: list push devices: %w", err)
	}
	if len(devices) == 0 {
		return errors.New("notifications: recipient has no active push device")
	}
	if len(devices) > 100 {
		devices = devices[:100]
	}
	requests := make([]expoPushRequest, 0, len(devices))
	for _, device := range devices {
		requests = append(requests, expoPushRequest{
			To: device.Token, Title: message.Subject, Body: message.Body,
			Sound: "default", Priority: "high",
			Data: map[string]any{"message_id": message.ID, "metadata": message.Metadata},
		})
	}
	payload, err := json.Marshal(requests)
	if err != nil {
		return fmt.Errorf("notifications: encode Expo push request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.config.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("notifications: create Expo push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if n.config.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.config.AccessToken)
	}
	response, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("notifications: Expo push request: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, response.Body.Close())
	}()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("notifications: read Expo push response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("notifications: Expo push returned HTTP %d", response.StatusCode)
	}
	var result expoPushResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("notifications: decode Expo push response: %w", err)
	}
	if len(result.Data) != len(requests) {
		return errors.New("notifications: Expo push returned an incomplete ticket set")
	}
	return n.processTickets(ctx, message, devices, result)
}

func (n *ExpoPushNotifier) processTickets(
	ctx context.Context,
	message domain.Message,
	devices []*domain.DeviceToken,
	result expoPushResponse,
) error {
	var deliveryErrors []string
	for index, ticket := range result.Data {
		if ticket.Status == "ok" {
			continue
		}
		if ticket.Details.Error == "DeviceNotRegistered" {
			if err := n.devices.MarkInvalid(ctx, message.TenantID, devices[index].Token); err != nil {
				deliveryErrors = append(deliveryErrors, "retire invalid device: "+err.Error())
				continue
			}
		}
		reason := strings.TrimSpace(ticket.Message)
		if reason == "" {
			reason = strings.TrimSpace(ticket.Details.Error)
		}
		if reason == "" {
			reason = "unknown provider error"
		}
		deliveryErrors = append(deliveryErrors, reason)
	}
	if len(deliveryErrors) > 0 {
		return fmt.Errorf("notifications: Expo push failed: %s", strings.Join(deliveryErrors, "; "))
	}
	return nil
}

var _ ports.Notifier = (*ExpoPushNotifier)(nil)
