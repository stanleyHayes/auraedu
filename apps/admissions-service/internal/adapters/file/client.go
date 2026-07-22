// Package file implements the private File Service ownership boundary.
package file

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/auraedu/admissions-service/internal/domain"
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
	return &Client{baseURL: config.ServiceURL(baseURL), token: token, http: &http.Client{Timeout: 5 * time.Second}}
}

func (c *Client) Verify(ctx context.Context, tenantID, applicantUserID, fileID string) error {
	if c.baseURL == "" || c.token == "" || tenantID == "" || applicantUserID == "" || fileID == "" {
		return domain.ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/v1/files/"+url.PathEscape(fileID)+"/ownership", nil)
	if err != nil {
		return domain.ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set(tenancy.HeaderTenantID, tenantID)
	res, err := c.http.Do(req)
	if err != nil {
		return domain.ErrUnavailable
	}
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			return
		}
	}()
	if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusUnauthorized {
		return domain.ErrForbidden
	}
	if res.StatusCode != http.StatusOK {
		return domain.ErrUnavailable
	}
	var metadata struct {
		FileID      string `json:"file_id"`
		OwnerID     string `json:"owner_id"`
		Status      string `json:"status"`
		ContentType string `json:"content_type"`
	}
	if err = httpx.DecodeJSONResponse(res.Body, &metadata); err != nil {
		return domain.ErrUnavailable
	}
	allowedType := metadata.ContentType == "application/pdf" || metadata.ContentType == "image/jpeg" || metadata.ContentType == "image/png"
	if metadata.FileID != fileID || metadata.OwnerID != applicantUserID || !strings.EqualFold(metadata.Status, "active") || !allowedType {
		return domain.ErrForbidden
	}
	return nil
}
