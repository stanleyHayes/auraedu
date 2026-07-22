// Package knowledge retrieves tenant-approved grounding passages.
package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/auraedu/ai-orchestrator-service/internal/domain"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/httpx"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{baseURL: config.ServiceURL(baseURL), token: token, http: &http.Client{Timeout: 8 * time.Second}}
}

func (c *Client) Search(ctx context.Context, tenantID, query, locale string, limit int, asOf time.Time) ([]domain.KnowledgeResult, error) {
	if c == nil || c.baseURL == "" || c.token == "" || strings.TrimSpace(tenantID) == "" {
		return nil, domain.ErrUnavailable
	}
	payload, err := json.Marshal(map[string]any{"query": query, "locale": locale, "limit": limit, "as_of": asOf.UTC()})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/knowledge/search", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-Tenant-Code", tenantID)
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("assistant knowledge search: %w", err)
	}
	defer response.Body.Close() //nolint:errcheck // Close errors cannot affect the completed response read.
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: knowledge service returned %d", domain.ErrUnavailable, response.StatusCode)
	}
	var result struct {
		Results []domain.KnowledgeResult `json:"results"`
	}
	if err := httpx.DecodeJSONResponse(response.Body, &result); err != nil {
		return nil, fmt.Errorf("%w: invalid knowledge response", domain.ErrUnavailable)
	}
	return result.Results, nil
}
