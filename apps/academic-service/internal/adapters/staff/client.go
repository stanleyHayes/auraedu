// Package staff implements the private Staff Service identity boundary.
package staff

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

func (c *Client) ResolveTeacher(ctx context.Context, tenantID, userID string) (string, error) {
	staffID, _, err := c.ResolveTeacherAssignments(ctx, tenantID, userID)
	return staffID, err
}

// ResolveTeacherAssignments resolves both the teacher identity and explicit
// staff-owned class scope in one authenticated request.
func (c *Client) ResolveTeacherAssignments(ctx context.Context, tenantID, userID string) (string, []string, error) {
	if c.baseURL == "" || c.token == "" {
		return "", nil, domain.ErrUnavailable
	}
	query := url.Values{"user_id": {userID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/v1/teacher-scope?"+query.Encode(), nil)
	if err != nil {
		return "", nil, domain.ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set(tenancy.HeaderTenantID, tenantID)
	res, err := c.http.Do(req)
	if err != nil {
		return "", nil, domain.ErrUnavailable
	}
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			return
		}
	}()
	switch res.StatusCode {
	case http.StatusNotFound:
		return "", nil, domain.ErrNotFound
	case http.StatusForbidden, http.StatusUnauthorized:
		return "", nil, domain.ErrForbidden
	case http.StatusOK:
	default:
		return "", nil, fmt.Errorf("%w: staff service returned %s", domain.ErrUnavailable, res.Status)
	}
	var body struct {
		StaffID  string   `json:"staff_id"`
		ClassIDs []string `json:"class_ids"`
	}
	if err := httpx.DecodeJSONResponse(res.Body, &body); err != nil || body.StaffID == "" {
		return "", nil, domain.ErrUnavailable
	}
	if body.ClassIDs == nil {
		body.ClassIDs = []string{}
	}
	return body.StaffID, body.ClassIDs, nil
}
