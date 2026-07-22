// Package memory provides deterministic in-memory adapters for tests and local use.
package memory

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/auraedu/ai-orchestrator-service/internal/domain"
)

type record struct {
	response    domain.Response
	requestHash string
}

type Repository struct {
	mu      sync.RWMutex
	records map[string]record
	actions map[string]domain.ActionProposal
	audit   map[string][]domain.ActionAuditEntry
}

func New() *Repository {
	return &Repository{
		records: map[string]record{},
		actions: map[string]domain.ActionProposal{},
		audit:   map[string][]domain.ActionAuditEntry{},
	}
}

func recordKey(tenantID, keyHash string) string { return tenantID + ":" + keyHash }

func (r *Repository) FindReplay(_ context.Context, tenantID, keyHash, _ string) (domain.Response, string, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, found := r.records[recordKey(tenantID, keyHash)]
	return record.response, record.requestHash, found, nil
}

func (r *Repository) Save(_ context.Context, response domain.Response, keyHash, requestHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := recordKey(response.TenantID, keyHash)
	if _, exists := r.records[key]; exists {
		return domain.ErrConflict
	}
	r.records[key] = record{response: response, requestHash: requestHash}
	return nil
}

func (r *Repository) PurgeExpired(context.Context) (int64, error) { return 0, nil }

func actionKey(tenantID, id string) string { return tenantID + ":" + id }

func (r *Repository) FindActionReplay(_ context.Context, tenantID, keyHash string) (domain.ActionProposal, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, action := range r.actions {
		if action.TenantID == tenantID && action.IdempotencyKeyHash == keyHash {
			return action, true, nil
		}
	}
	return domain.ActionProposal{}, false, nil
}

func (r *Repository) CreateAction(_ context.Context, action domain.ActionProposal, audit domain.ActionAuditEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.actions {
		if existing.TenantID == action.TenantID && existing.IdempotencyKeyHash == action.IdempotencyKeyHash {
			return domain.ErrConflict
		}
	}
	key := actionKey(action.TenantID, action.ID)
	r.actions[key] = action
	r.audit[key] = append(r.audit[key], audit)
	return nil
}

func (r *Repository) ListActions(_ context.Context, tenantID, status string, limit int) ([]domain.ActionProposal, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := []domain.ActionProposal{}
	for _, action := range r.actions {
		if action.TenantID == tenantID && (status == "" || string(action.Status) == status) {
			items = append(items, action)
			if len(items) == limit {
				break
			}
		}
	}
	return items, nil
}

func (r *Repository) GetAction(_ context.Context, tenantID, id string) (domain.ActionProposal, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	action, ok := r.actions[actionKey(tenantID, id)]
	if !ok {
		return domain.ActionProposal{}, domain.ErrNotFound
	}
	return action, nil
}

func (r *Repository) ReviewAction(
	_ context.Context,
	tenantID, id, reviewerID, reviewerRole, note string,
	approve bool,
	now time.Time,
) (domain.ActionProposal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := actionKey(tenantID, id)
	action, ok := r.actions[key]
	if !ok {
		return domain.ActionProposal{}, domain.ErrNotFound
	}
	if action.Status != domain.ActionPending || action.ProposedBy == reviewerID {
		return domain.ActionProposal{}, domain.ErrInvalidState
	}
	status, event := domain.ActionRejected, "rejected"
	if approve {
		status, event = domain.ActionApproved, "approved"
	}
	action.Status = status
	action.ReviewedBy = &reviewerID
	action.ReviewerRole = &reviewerRole
	action.ReviewNote = &note
	action.ReviewedAt = &now
	action.UpdatedAt = now
	r.actions[key] = action
	evidence, err := json.Marshal(map[string]string{"note": note})
	if err != nil {
		return domain.ActionProposal{}, err
	}
	r.audit[key] = append(r.audit[key], domain.ActionAuditEntry{
		ActionID:   id,
		Event:      event,
		ActorID:    reviewerID,
		ActorRole:  reviewerRole,
		Evidence:   evidence,
		OccurredAt: now,
	})
	return action, nil
}

func (r *Repository) StartActionExecution(_ context.Context, tenantID, id, actorID, actorRole string, now time.Time) (domain.ActionProposal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := actionKey(tenantID, id)
	action, ok := r.actions[key]
	if !ok {
		return domain.ActionProposal{}, domain.ErrNotFound
	}
	if action.Status != domain.ActionApproved && action.Status != domain.ActionFailed {
		return domain.ActionProposal{}, domain.ErrInvalidState
	}
	action.Status, action.ExecutionAttempts, action.UpdatedAt = domain.ActionExecuting, action.ExecutionAttempts+1, now
	r.actions[key] = action
	r.audit[key] = append(r.audit[key], domain.ActionAuditEntry{
		ActionID:   id,
		Event:      "execution_started",
		ActorID:    actorID,
		ActorRole:  actorRole,
		OccurredAt: now,
	})
	return action, nil
}

func (r *Repository) FinishActionExecution(
	_ context.Context,
	tenantID, id string,
	succeeded bool,
	result json.RawMessage,
	code, detail string,
	now time.Time,
) (domain.ActionProposal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := actionKey(tenantID, id)
	action, ok := r.actions[key]
	if !ok {
		return domain.ActionProposal{}, domain.ErrNotFound
	}
	if action.Status != domain.ActionExecuting {
		return domain.ActionProposal{}, domain.ErrInvalidState
	}
	status, event := domain.ActionFailed, "execution_failed"
	if succeeded {
		status, event = domain.ActionSucceeded, "execution_succeeded"
	}
	action.Status, action.Result, action.ExecutedAt, action.UpdatedAt = status, result, &now, now
	if code != "" {
		action.FailureCode = &code
	}
	if detail != "" {
		action.FailureDetail = &detail
	}
	r.actions[key] = action
	r.audit[key] = append(r.audit[key], domain.ActionAuditEntry{
		ActionID:   id,
		Event:      event,
		ActorID:    "ai-orchestrator-service",
		ActorRole:  "service",
		OccurredAt: now,
	})
	return action, nil
}

func (r *Repository) ListActionAudit(_ context.Context, tenantID, id string) ([]domain.ActionAuditEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]domain.ActionAuditEntry(nil), r.audit[actionKey(tenantID, id)]...), nil
}
