// Package student implements the private Student Service learner-scope boundary.
package student

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
)

type Client struct {
	baseURL, token string
	http           *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{baseURL: config.ServiceURL(baseURL), token: token, http: &http.Client{Timeout: 5 * time.Second}}
}
func (c *Client) Resolve(ctx context.Context, tenantID, userID, role string) ([]string, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, domain.ErrUnavailable
	}
	query := url.Values{"user_id": {userID}, "role": {role}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/v1/learner-scope?"+query.Encode(), nil)
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set(tenancy.HeaderTenantID, tenantID)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			return
		}
	}()
	if res.StatusCode == http.StatusNotFound {
		return []string{}, nil
	}
	if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusUnauthorized {
		return nil, domain.ErrForbidden
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: student service returned %s", domain.ErrUnavailable, res.Status)
	}
	var body struct {
		ClassIDs []string `json:"class_ids"`
	}
	if httpx.DecodeJSONResponse(res.Body, &body) != nil {
		return nil, domain.ErrUnavailable
	}
	if body.ClassIDs == nil {
		body.ClassIDs = []string{}
	}
	return body.ClassIDs, nil
}
