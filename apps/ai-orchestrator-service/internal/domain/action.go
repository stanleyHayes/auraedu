package domain

import (
	"encoding/json"
	"time"
)

const (
	ActionCRMAssignLead = "crm.lead.assign"
	ActionPolicyVersion = "2026-07-19.v1"
	ActionLevelLowRisk  = 2
)

type ActionStatus string

const (
	ActionPending   ActionStatus = "pending_approval"
	ActionApproved  ActionStatus = "approved"
	ActionExecuting ActionStatus = "executing"
	ActionSucceeded ActionStatus = "succeeded"
	ActionFailed    ActionStatus = "failed"
	ActionRejected  ActionStatus = "rejected"
	ActionCancelled ActionStatus = "cancelled"
)

type ActionProposal struct {
	ID                 string          `json:"id"`
	TenantID           string          `json:"tenant_id,omitempty"`
	Action             string          `json:"action"`
	Level              int             `json:"level"`
	PolicyVersion      string          `json:"policy_version"`
	TargetType         string          `json:"target_type"`
	TargetID           string          `json:"target_id"`
	Payload            json.RawMessage `json:"payload"`
	PayloadHash        string          `json:"payload_hash"`
	Reason             string          `json:"reason"`
	Status             ActionStatus    `json:"status"`
	ProposedBy         string          `json:"proposed_by"`
	ProposerRole       string          `json:"proposer_role"`
	ReviewedBy         *string         `json:"reviewed_by"`
	ReviewerRole       *string         `json:"reviewer_role"`
	ReviewNote         *string         `json:"review_note"`
	ReviewedAt         *time.Time      `json:"reviewed_at"`
	ExecutionAttempts  int             `json:"execution_attempts"`
	Result             json.RawMessage `json:"result,omitempty"`
	FailureCode        *string         `json:"failure_code"`
	FailureDetail      *string         `json:"failure_detail"`
	ExecutedAt         *time.Time      `json:"executed_at"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	IdempotencyKeyHash string          `json:"-"`
	RequestHash        string          `json:"-"`
}

type ActionAuditEntry struct {
	ID         string          `json:"id"`
	ActionID   string          `json:"action_id"`
	Event      string          `json:"event"`
	ActorID    string          `json:"actor_id"`
	ActorRole  string          `json:"actor_role"`
	Evidence   json.RawMessage `json:"evidence"`
	OccurredAt time.Time       `json:"occurred_at"`
}

type ActionExecutionResult struct {
	StatusCode int             `json:"status_code"`
	Body       json.RawMessage `json:"body"`
}
