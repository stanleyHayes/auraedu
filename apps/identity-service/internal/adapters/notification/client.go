// Package notification delivers identity lifecycle email through notification-service.
package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/auraedu/platform/config"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{baseURL: config.ServiceURL(baseURL), token: token, http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) Deliver(ctx context.Context, tenantID, recipient, template string, data map[string]any) (returnErr error) {
	body, err := json.Marshal(map[string]any{"tenant_id": tenantID, "recipient": recipient, "template": template, "data": data})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/transactional-email", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("notification delivery: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, resp.Body.Close()) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if readErr != nil {
			return fmt.Errorf("notification delivery: read error response: %w", readErr)
		}
		return fmt.Errorf("notification delivery: status %d: %s", resp.StatusCode, strings.TrimSpace(string(limited)))
	}
	return nil
}
