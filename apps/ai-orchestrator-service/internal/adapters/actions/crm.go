// Package actions executes approved AI actions against downstream services.
package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/auraedu/ai-orchestrator-service/internal/domain"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/tenancy"
)

type CRMExecutor struct {
	baseURL string
	client  *http.Client
}

func NewCRMExecutor(baseURL string) *CRMExecutor {
	return &CRMExecutor{baseURL: config.ServiceURL(baseURL), client: &http.Client{Timeout: 10 * time.Second}}
}

func (e *CRMExecutor) Execute(ctx context.Context, action domain.ActionProposal, actor auth.Actor) (domain.ActionExecutionResult, error) {
	if action.Action != domain.ActionCRMAssignLead {
		return domain.ActionExecutionResult{}, domain.ErrProhibited
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, e.baseURL+"/api/v1/leads/"+action.TargetID, bytes.NewReader(action.Payload))
	if err != nil {
		return domain.ActionExecutionResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(tenancy.HeaderTenantID, action.TenantID)
	req.Header.Set(auth.HeaderTenant, action.TenantID)
	req.Header.Set(auth.HeaderUserID, actor.UserID)
	req.Header.Set(auth.HeaderRole, actor.Role)
	req.Header.Set(auth.HeaderPermissions, strings.Join(actor.Permissions, ","))
	req.Header.Set("Idempotency-Key", action.ID+":"+action.PayloadHash[:16])
	response, err := e.client.Do(req)
	if err != nil {
		return domain.ActionExecutionResult{}, err
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			return
		}
	}()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return domain.ActionExecutionResult{StatusCode: response.StatusCode}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		redacted, marshalErr := json.Marshal(map[string]bool{"response_redacted": true})
		if marshalErr != nil {
			return domain.ActionExecutionResult{StatusCode: response.StatusCode}, marshalErr
		}
		return domain.ActionExecutionResult{StatusCode: response.StatusCode, Body: redacted}, fmt.Errorf("CRM returned HTTP %d", response.StatusCode)
	}
	// CRM returns the full lead aggregate. Action evidence must never duplicate
	// prospect PII, so retain only the resource and mutation identifiers.
	var changed struct {
		ID          string  `json:"id"`
		OwnerUserID *string `json:"owner_user_id"`
	}
	if err := json.Unmarshal(body, &changed); err != nil || changed.ID == "" || changed.OwnerUserID == nil {
		return domain.ActionExecutionResult{StatusCode: response.StatusCode}, fmt.Errorf("CRM returned an invalid assignment response")
	}
	sanitized, err := json.Marshal(map[string]string{
		"lead_id":       changed.ID,
		"owner_user_id": *changed.OwnerUserID,
	})
	if err != nil {
		return domain.ActionExecutionResult{StatusCode: response.StatusCode}, err
	}
	return domain.ActionExecutionResult{StatusCode: response.StatusCode, Body: sanitized}, nil
}
