// Package student resolves authorized learner identities from Student Service.
package student

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/auraedu/cbt-service/internal/domain"
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

func (c *Client) ResolveStudentIDs(ctx context.Context, tenantID, userID, role string) ([]string, error) {
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
	response, err := c.http.Do(req)
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	defer response.Body.Close() //nolint:errcheck // Close errors cannot affect a completed response read.
	if response.StatusCode == http.StatusNotFound {
		return []string{}, nil
	}
	if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusUnauthorized {
		return nil, domain.ErrForbidden
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: student service returned %s", domain.ErrUnavailable, response.Status)
	}
	var body struct {
		StudentIDs []string `json:"student_ids"`
	}
	if httpx.DecodeJSONResponse(response.Body, &body) != nil {
		return nil, domain.ErrUnavailable
	}
	if body.StudentIDs == nil {
		body.StudentIDs = []string{}
	}
	return body.StudentIDs, nil
}
