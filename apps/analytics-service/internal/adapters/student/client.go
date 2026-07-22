// Package student resolves role-scoped learner access through the authenticated
// Student Service internal API.
package student

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/auraedu/analytics-service/internal/ports"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: config.ServiceURL(baseURL),
		token:   token,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) Resolve(ctx context.Context, tenantID, userID, role string) (ports.LearnerScope, error) {
	if c == nil || c.baseURL == "" || c.token == "" {
		return ports.LearnerScope{}, domain.ErrUnavailable
	}
	query := url.Values{"user_id": {userID}, "role": {role}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/v1/learner-scope?"+query.Encode(), nil)
	if err != nil {
		return ports.LearnerScope{}, domain.ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set(tenancy.HeaderTenantID, tenantID)
	res, err := c.http.Do(req)
	if err != nil {
		return ports.LearnerScope{}, domain.ErrUnavailable
	}
	defer res.Body.Close() //nolint:errcheck // Close errors cannot affect a completed response read.
	if res.StatusCode == http.StatusNotFound {
		return ports.LearnerScope{StudentIDs: []string{}}, nil
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return ports.LearnerScope{}, domain.ErrForbidden
	}
	if res.StatusCode != http.StatusOK {
		return ports.LearnerScope{}, fmt.Errorf("%w: student service returned %s", domain.ErrUnavailable, res.Status)
	}
	var payload struct {
		StudentIDs []string `json:"student_ids"`
	}
	if err := httpx.DecodeJSONResponse(res.Body, &payload); err != nil {
		return ports.LearnerScope{}, domain.ErrUnavailable
	}
	if payload.StudentIDs == nil {
		payload.StudentIDs = []string{}
	}
	return ports.LearnerScope{StudentIDs: payload.StudentIDs}, nil
}

var _ ports.LearnerScopeResolver = (*Client)(nil)
