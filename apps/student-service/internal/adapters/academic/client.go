// Package academic resolves authoritative teacher class scope through Academic Service.
package academic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/student-service/internal/domain"
)

type Client struct {
	baseURL, token string
	http           *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{baseURL: config.ServiceURL(baseURL), token: token, http: &http.Client{Timeout: 5 * time.Second}}
}

func (c *Client) ResolveTeacherClasses(
	ctx context.Context,
	tenantID string,
	userID string,
) (classIDs []string, returnErr error) {
	if c.baseURL == "" || c.token == "" {
		return nil, domain.ErrUnavailable
	}
	query := url.Values{"user_id": {userID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/v1/teacher-class-scope?"+query.Encode(), nil)
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
		returnErr = errors.Join(returnErr, res.Body.Close())
	}()
	switch res.StatusCode {
	case http.StatusNotFound:
		return []string{}, nil
	case http.StatusForbidden, http.StatusUnauthorized:
		return nil, domain.ErrForbidden
	case http.StatusOK:
	default:
		return nil, fmt.Errorf("%w: academic service returned %s", domain.ErrUnavailable, res.Status)
	}
	var body struct {
		ClassIDs []string `json:"class_ids"`
	}
	if err := httpx.DecodeJSONResponse(res.Body, &body); err != nil {
		return nil, domain.ErrUnavailable
	}
	if body.ClassIDs == nil {
		body.ClassIDs = []string{}
	}
	return body.ClassIDs, nil
}
