// Package fees resolves authoritative invoice ownership through Fees Service.
package fees

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
)

// Client resolves learner-owned invoice access through the private Fees API.
type Client struct {
	baseURL, token string
	http           *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{baseURL: config.ServiceURL(baseURL), token: token, http: &http.Client{Timeout: 5 * time.Second}}
}

func (c *Client) Resolve(
	ctx context.Context,
	tenantID string,
	userID string,
	role string,
	invoiceIDs []string,
) (invoiceIDsResult []string, returnErr error) {
	if c.baseURL == "" || c.token == "" {
		return nil, domain.ErrUnavailable
	}
	payload, err := json.Marshal(map[string]any{"user_id": userID, "role": role, "invoice_ids": invoiceIDs})
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/invoice-access", bytes.NewReader(payload))
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(tenancy.HeaderTenantID, tenantID)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	defer func() {
		returnErr = errors.Join(returnErr, res.Body.Close())
	}()
	if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusUnauthorized {
		return nil, domain.ErrForbidden
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: fees service returned %s", domain.ErrUnavailable, res.Status)
	}
	var body struct {
		InvoiceIDs []string `json:"invoice_ids"`
	}
	if err := httpx.DecodeJSONResponse(res.Body, &body); err != nil {
		return nil, domain.ErrUnavailable
	}
	if body.InvoiceIDs == nil {
		body.InvoiceIDs = []string{}
	}
	return body.InvoiceIDs, nil
}
