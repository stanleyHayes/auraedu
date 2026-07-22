// Package student resolves authorized learner scopes from Student Service.
package student

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
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

func (c *Client) Resolve(ctx context.Context, tenantID, userID, role string) (ports.LearnerScope, error) {
	if c.baseURL == "" || c.token == "" {
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
		return ports.LearnerScope{StudentIDs: []string{}, ClassIDs: []string{}}, nil
	}
	if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusUnauthorized {
		return ports.LearnerScope{}, domain.ErrForbidden
	}
	if res.StatusCode != http.StatusOK {
		return ports.LearnerScope{}, fmt.Errorf("%w: student service returned %s", domain.ErrUnavailable, res.Status)
	}
	var body struct {
		StudentIDs []string `json:"student_ids"`
		ClassIDs   []string `json:"class_ids"`
	}
	if err := httpx.DecodeJSONResponse(res.Body, &body); err != nil {
		return ports.LearnerScope{}, domain.ErrUnavailable
	}
	if body.StudentIDs == nil {
		body.StudentIDs = []string{}
	}
	if body.ClassIDs == nil {
		body.ClassIDs = []string{}
	}
	return ports.LearnerScope{StudentIDs: body.StudentIDs, ClassIDs: body.ClassIDs}, nil
}
