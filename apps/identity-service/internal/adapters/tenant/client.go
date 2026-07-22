// Package tenant resolves identity tenant state through tenant-service.
package tenant

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/httpx"
)

type OnboardingAdministrator struct {
	TenantCode        string `json:"tenant_code"`
	AdministratorName string `json:"administrator_name"`
	Email             string `json:"email"`
}

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: config.ServiceURL(baseURL), token: token,
		http: &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *Client) ResolveOnboardingAdministrator(ctx context.Context, requestID string) (_ OnboardingAdministrator, returnErr error) {
	if c == nil || c.baseURL == "" || c.token == "" || strings.TrimSpace(requestID) == "" {
		return OnboardingAdministrator{}, fmt.Errorf("identity onboarding: tenant resolver is not configured")
	}
	endpoint := c.baseURL + "/internal/v1/onboarding-requests/" + url.PathEscape(requestID) + "/administrator"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return OnboardingAdministrator{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	response, err := c.http.Do(req)
	if err != nil {
		return OnboardingAdministrator{}, fmt.Errorf("identity onboarding: resolve administrator: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, response.Body.Close()) }()
	if response.StatusCode != http.StatusOK {
		return OnboardingAdministrator{}, fmt.Errorf("identity onboarding: tenant resolver returned %d", response.StatusCode)
	}
	var administrator OnboardingAdministrator
	if err := httpx.DecodeJSONResponse(response.Body, &administrator); err != nil {
		return OnboardingAdministrator{}, err
	}
	if administrator.TenantCode == "" || administrator.AdministratorName == "" || administrator.Email == "" {
		return OnboardingAdministrator{}, fmt.Errorf("identity onboarding: incomplete administrator response")
	}
	return administrator, nil
}

func (c *Client) Activate(ctx context.Context, tenantID string) (returnErr error) {
	if c == nil || c.baseURL == "" || c.token == "" || strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("identity activation: tenant client is not configured")
	}
	endpoint := c.baseURL + "/internal/v1/tenants/" + url.PathEscape(tenantID) + "/activate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("identity activation: activate tenant: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, response.Body.Close()) }()
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("identity activation: tenant service returned %d", response.StatusCode)
	}
	return nil
}
