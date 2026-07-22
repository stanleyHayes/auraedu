// Package crm resolves consented notification recipients through CRM.
package crm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/httpx"

	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{baseURL: config.ServiceURL(baseURL), token: token, http: &http.Client{Timeout: 5 * time.Second}}
}

func (c *Client) ResolveWelcomeRecipient(ctx context.Context, tenantID, leadID string) (_ ports.LeadWelcomeRecipient, returnErr error) {
	var recipient ports.LeadWelcomeRecipient
	if c.baseURL == "" || c.token == "" {
		return recipient, fmt.Errorf("notifications: CRM resolver is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/v1/leads/"+url.PathEscape(leadID)+"/welcome-recipient", nil)
	if err != nil {
		return recipient, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set(tenancy.HeaderTenantID, tenantID)
	resp, err := c.http.Do(req)
	if err != nil {
		return recipient, fmt.Errorf("notifications: resolve CRM lead: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, resp.Body.Close()) }()
	if resp.StatusCode != http.StatusOK {
		return recipient, fmt.Errorf("notifications: CRM resolver returned %s", resp.Status)
	}
	if err := httpx.DecodeJSONResponse(resp.Body, &recipient); err != nil {
		return recipient, fmt.Errorf("notifications: decode CRM recipient: %w", err)
	}
	return recipient, nil
}
