package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/auraedu/ai-orchestrator-service/internal/domain"
	"github.com/auraedu/ai-orchestrator-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
)

const (
	FeatureAutonomousActions = "growth_autonomous_actions"
	PermConfigureAgent       = "ai.agent.configure"
	PermApproveAction        = "ai.action.approve"
	PermAssignLead           = "crm.lead.assign"
)

type ActionService struct {
	repo     ports.ActionRepository
	executor ports.ActionExecutor
	gate     flags.Gate
	now      func() time.Time
}

func NewActionService(repo ports.ActionRepository, executor ports.ActionExecutor, gate flags.Gate) *ActionService {
	return &ActionService{repo: repo, executor: executor, gate: gate, now: time.Now}
}

type ProposeActionInput struct {
	Action, TargetID, Reason, IdempotencyKey string
	Payload                                  json.RawMessage
}

type leadAssignmentPayload struct {
	OwnerUserID string `json:"owner_user_id"`
}

func (s *ActionService) Propose(ctx context.Context, actor auth.Actor, input ProposeActionInput) (domain.ActionProposal, error) {
	tenantID, err := s.require(ctx, actor, PermConfigureAgent)
	if err != nil {
		return domain.ActionProposal{}, err
	}
	input.Action = strings.TrimSpace(input.Action)
	input.TargetID = strings.TrimSpace(input.TargetID)
	input.Reason = strings.TrimSpace(input.Reason)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 || len(input.Reason) < 10 || len(input.Reason) > 1000 {
		return domain.ActionProposal{}, domain.ErrValidation
	}
	if input.Action != domain.ActionCRMAssignLead {
		return domain.ActionProposal{}, domain.ErrProhibited
	}
	if _, err := uuid.Parse(input.TargetID); err != nil {
		return domain.ActionProposal{}, domain.ErrValidation
	}
	payload, err := parseLeadAssignment(input.Payload)
	if err != nil {
		return domain.ActionProposal{}, err
	}
	canonical, err := json.Marshal(payload)
	if err != nil {
		return domain.ActionProposal{}, err
	}
	requestBytes, err := json.Marshal(map[string]any{
		"action":    input.Action,
		"target_id": input.TargetID,
		"payload":   payload,
		"reason":    input.Reason,
	})
	if err != nil {
		return domain.ActionProposal{}, err
	}
	now := s.now().UTC()
	action := domain.ActionProposal{
		ID:                 uuid.NewString(),
		TenantID:           tenantID,
		Action:             input.Action,
		Level:              domain.ActionLevelLowRisk,
		PolicyVersion:      domain.ActionPolicyVersion,
		TargetType:         "crm_lead",
		TargetID:           input.TargetID,
		Payload:            canonical,
		PayloadHash:        actionDigest(canonical),
		Reason:             input.Reason,
		Status:             domain.ActionPending,
		ProposedBy:         actor.UserID,
		ProposerRole:       actor.Role,
		IdempotencyKeyHash: actionDigest([]byte(input.IdempotencyKey)),
		RequestHash:        actionDigest(requestBytes),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if replay, found, err := s.repo.FindActionReplay(ctx, tenantID, action.IdempotencyKeyHash); err != nil {
		return domain.ActionProposal{}, err
	} else if found {
		if replay.RequestHash != action.RequestHash {
			return domain.ActionProposal{}, domain.ErrConflict
		}
		return replay, nil
	}
	evidence, err := json.Marshal(map[string]any{
		"payload_hash":   action.PayloadHash,
		"policy_version": action.PolicyVersion,
		"level":          action.Level,
		"reason":         action.Reason,
	})
	if err != nil {
		return domain.ActionProposal{}, err
	}
	audit := domain.ActionAuditEntry{
		ID:         uuid.NewString(),
		ActionID:   action.ID,
		Event:      "proposed",
		ActorID:    actor.UserID,
		ActorRole:  actor.Role,
		Evidence:   evidence,
		OccurredAt: now,
	}
	if err := s.repo.CreateAction(ctx, action, audit); err != nil {
		replay, found, replayErr := s.repo.FindActionReplay(ctx, tenantID, action.IdempotencyKeyHash)
		if replayErr == nil && found && replay.RequestHash == action.RequestHash {
			return replay, nil
		}
		return domain.ActionProposal{}, err
	}
	return action, nil
}

func parseLeadAssignment(raw json.RawMessage) (leadAssignmentPayload, error) {
	var payload leadAssignmentPayload
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return leadAssignmentPayload{}, domain.ErrValidation
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return leadAssignmentPayload{}, domain.ErrValidation
	}
	payload.OwnerUserID = strings.TrimSpace(payload.OwnerUserID)
	if _, err := uuid.Parse(payload.OwnerUserID); err != nil {
		return leadAssignmentPayload{}, domain.ErrValidation
	}
	return payload, nil
}

func (s *ActionService) List(ctx context.Context, actor auth.Actor, status string, limit int) ([]domain.ActionProposal, error) {
	tenantID, err := s.require(ctx, actor, PermConfigureAgent)
	if err != nil {
		return nil, err
	}
	status = strings.TrimSpace(status)
	if status != "" && !validActionStatus(domain.ActionStatus(status)) {
		return nil, domain.ErrValidation
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListActions(ctx, tenantID, status, limit)
}

func (s *ActionService) Get(ctx context.Context, actor auth.Actor, id string) (domain.ActionProposal, []domain.ActionAuditEntry, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return domain.ActionProposal{}, nil, domain.ErrValidation
	}
	tenantID, err := s.require(ctx, actor, PermConfigureAgent)
	if err != nil {
		return domain.ActionProposal{}, nil, err
	}
	action, err := s.repo.GetAction(ctx, tenantID, id)
	if err != nil {
		return domain.ActionProposal{}, nil, err
	}
	audit, err := s.repo.ListActionAudit(ctx, tenantID, id)
	return action, audit, err
}

func (s *ActionService) Review(ctx context.Context, actor auth.Actor, id, note string, approve bool) (domain.ActionProposal, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return domain.ActionProposal{}, domain.ErrValidation
	}
	tenantID, err := s.requireReviewer(ctx, actor)
	if err != nil {
		return domain.ActionProposal{}, err
	}
	current, err := s.repo.GetAction(ctx, tenantID, id)
	if err != nil {
		return domain.ActionProposal{}, err
	}
	if current.Status != domain.ActionPending {
		return domain.ActionProposal{}, domain.ErrInvalidState
	}
	if current.ProposedBy == actor.UserID || !humanRole(actor.Role) {
		return domain.ActionProposal{}, domain.ErrForbidden
	}
	note = strings.TrimSpace(note)
	if !approve && len(note) < 3 {
		return domain.ActionProposal{}, domain.ErrValidation
	}
	action, err := s.repo.ReviewAction(ctx, tenantID, id, actor.UserID, actor.Role, note, approve, s.now().UTC())
	if err != nil {
		return domain.ActionProposal{}, err
	}
	if !approve {
		return action, nil
	}
	return s.execute(ctx, actor, action.ID)
}

func (s *ActionService) Retry(ctx context.Context, actor auth.Actor, id string) (domain.ActionProposal, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return domain.ActionProposal{}, domain.ErrValidation
	}
	tenantID, err := s.requireReviewer(ctx, actor)
	if err != nil {
		return domain.ActionProposal{}, err
	}
	current, err := s.repo.GetAction(ctx, tenantID, id)
	if err != nil {
		return domain.ActionProposal{}, err
	}
	if current.Status != domain.ActionFailed || !humanRole(actor.Role) {
		return domain.ActionProposal{}, domain.ErrInvalidState
	}
	return s.execute(ctx, actor, id)
}

func (s *ActionService) execute(ctx context.Context, actor auth.Actor, id string) (domain.ActionProposal, error) {
	tenantID := tenancy.TenantID(ctx)
	action, err := s.repo.StartActionExecution(ctx, tenantID, id, actor.UserID, actor.Role, s.now().UTC())
	if err != nil {
		return domain.ActionProposal{}, err
	}
	result, execErr := s.executor.Execute(ctx, action, actor)
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return domain.ActionProposal{}, err
	}
	if execErr != nil {
		detail := truncate(execErr.Error(), 1000)
		failed, persistErr := s.repo.FinishActionExecution(
			ctx, tenantID, id, false, resultBytes, "downstream_rejected", detail, s.now().UTC(),
		)
		if persistErr != nil {
			return domain.ActionProposal{}, persistErr
		}
		return failed, nil
	}
	return s.repo.FinishActionExecution(ctx, tenantID, id, true, resultBytes, "", "", s.now().UTC())
}

func (s *ActionService) require(ctx context.Context, actor auth.Actor, permission string) (string, error) {
	tenantID := strings.TrimSpace(tenancy.TenantID(ctx))
	if tenantID == "" || !actor.Authenticated() || !actor.CanAccessTenant(tenantID) || !actor.Has(permission) {
		return "", domain.ErrForbidden
	}
	if s.gate != nil && !s.gate.IsEnabled(ctx, tenantID, FeatureAutonomousActions) {
		return "", domain.ErrForbidden
	}
	return tenantID, nil
}

func (s *ActionService) requireReviewer(ctx context.Context, actor auth.Actor) (string, error) {
	tenantID, err := s.require(ctx, actor, PermApproveAction)
	if err != nil {
		return "", err
	}
	if !actor.Has(PermAssignLead) {
		return "", domain.ErrForbidden
	}
	return tenantID, nil
}

func humanRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	return role != "" && !strings.Contains(role, "ai") && !strings.Contains(role, "agent") && !strings.Contains(role, "service") && role != "system"
}

func validActionStatus(status domain.ActionStatus) bool {
	switch status {
	case domain.ActionPending,
		domain.ActionApproved,
		domain.ActionExecuting,
		domain.ActionSucceeded,
		domain.ActionFailed,
		domain.ActionRejected,
		domain.ActionCancelled:
		return true
	}
	return false
}

func actionDigest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
