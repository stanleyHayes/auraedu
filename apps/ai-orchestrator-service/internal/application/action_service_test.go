package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/auraedu/ai-orchestrator-service/internal/adapters/memory"
	"github.com/auraedu/ai-orchestrator-service/internal/application"
	"github.com/auraedu/ai-orchestrator-service/internal/domain"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

const (
	testLeadID  = "11111111-1111-4111-8111-111111111111"
	testOwnerID = "22222222-2222-4222-8222-222222222222"
)

type actionExecutor struct {
	calls int
	fail  bool
	seen  domain.ActionProposal
}

func (e *actionExecutor) Execute(_ context.Context, action domain.ActionProposal, _ auth.Actor) (domain.ActionExecutionResult, error) {
	e.calls++
	e.seen = action
	if e.fail {
		return domain.ActionExecutionResult{StatusCode: 503}, errors.New("CRM unavailable")
	}
	return domain.ActionExecutionResult{StatusCode: 200, Body: json.RawMessage(`{"id":"` + testLeadID + `","owner_user_id":"` + testOwnerID + `"}`)}, nil
}

func actionTestContext(actor auth.Actor) context.Context {
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: "school-one", ActorID: actor.UserID, ActorRole: actor.Role})
	return auth.WithActor(ctx, actor)
}

func actionActors() (auth.Actor, auth.Actor) {
	proposer := auth.Actor{UserID: "ai-planner-1", TenantID: "school-one", Role: "ai_agent", Permissions: []string{application.PermConfigureAgent}}
	reviewer := auth.Actor{UserID: "admin-2", TenantID: "school-one", Role: "school_admin", Permissions: []string{application.PermConfigureAgent, application.PermApproveAction, application.PermAssignLead}}
	return proposer, reviewer
}

func newActionService(executor *actionExecutor) *application.ActionService {
	gate := flags.NewStaticSnapshot()
	gate.Set("school-one", application.FeatureAutonomousActions, true)
	return application.NewActionService(memory.New(), executor, gate)
}

func proposeAllowed(t *testing.T, service *application.ActionService, actor auth.Actor) domain.ActionProposal {
	t.Helper()
	action, err := service.Propose(actionTestContext(actor), actor, application.ProposeActionInput{Action: domain.ActionCRMAssignLead, TargetID: testLeadID,
		Payload: json.RawMessage(`{"owner_user_id":"` + testOwnerID + `"}`), Reason: "Assign this qualified lead to the configured regional owner.", IdempotencyKey: "controlled-action-test-key-0001"})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	return action
}

func TestControlledActionRequiresIndependentHumanApprovalAndExecutesAllowlist(t *testing.T) {
	executor := &actionExecutor{}
	service := newActionService(executor)
	proposer, reviewer := actionActors()
	action := proposeAllowed(t, service, proposer)
	if action.Status != domain.ActionPending || action.Level != 2 || action.PolicyVersion == "" || action.PayloadHash == "" {
		t.Fatalf("proposal=%+v", action)
	}
	if _, err := service.Review(actionTestContext(proposer), proposer, action.ID, "self approval", true); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("AI self approval error=%v", err)
	}
	aiReviewer := proposer
	aiReviewer.UserID = "ai-reviewer-2"
	aiReviewer.Permissions = []string{application.PermApproveAction, application.PermAssignLead}
	if _, err := service.Review(actionTestContext(aiReviewer), aiReviewer, action.ID, "AI approval", true); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("AI approval error=%v", err)
	}
	completed, err := service.Review(actionTestContext(reviewer), reviewer, action.ID, "Owner is on duty and has capacity.", true)
	if err != nil || completed.Status != domain.ActionSucceeded || completed.ExecutionAttempts != 1 || executor.calls != 1 {
		t.Fatalf("completed=%+v calls=%d err=%v", completed, executor.calls, err)
	}
	_, audit, err := service.Get(actionTestContext(reviewer), reviewer, action.ID)
	if err != nil || len(audit) != 4 || audit[0].Event != "proposed" || audit[3].Event != "execution_succeeded" {
		t.Fatalf("audit=%+v err=%v", audit, err)
	}
}

func TestControlledActionFailsClosedForUnknownAndReplayChanges(t *testing.T) {
	service := newActionService(&actionExecutor{})
	proposer, _ := actionActors()
	input := application.ProposeActionInput{Action: "admissions.offer.issue", TargetID: testLeadID, Payload: json.RawMessage(`{}`), Reason: "Attempt a prohibited admission decision action.", IdempotencyKey: "controlled-action-denied-0001"}
	if _, err := service.Propose(actionTestContext(proposer), proposer, input); !errors.Is(err, domain.ErrProhibited) {
		t.Fatalf("prohibited error=%v", err)
	}
	first := proposeAllowed(t, service, proposer)
	replay := proposeAllowed(t, service, proposer)
	if replay.ID != first.ID {
		t.Fatalf("replay ids differ: %s %s", first.ID, replay.ID)
	}
	_, err := service.Propose(actionTestContext(proposer), proposer, application.ProposeActionInput{Action: domain.ActionCRMAssignLead, TargetID: testLeadID,
		Payload: json.RawMessage(`{"owner_user_id":"33333333-3333-4333-8333-333333333333"}`), Reason: "Assign this qualified lead to a different configured owner.", IdempotencyKey: "controlled-action-test-key-0001"})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("changed replay error=%v", err)
	}
}

func TestControlledActionPersistsFailureAndRetriesOnce(t *testing.T) {
	executor := &actionExecutor{fail: true}
	service := newActionService(executor)
	proposer, reviewer := actionActors()
	action := proposeAllowed(t, service, proposer)
	failed, err := service.Review(actionTestContext(reviewer), reviewer, action.ID, "Owner assignment reviewed.", true)
	if err != nil || failed.Status != domain.ActionFailed || failed.FailureDetail == nil || failed.ExecutionAttempts != 1 {
		t.Fatalf("failed=%+v err=%v", failed, err)
	}
	executor.fail = false
	completed, err := service.Retry(actionTestContext(reviewer), reviewer, action.ID)
	if err != nil || completed.Status != domain.ActionSucceeded || completed.ExecutionAttempts != 2 || executor.calls != 2 {
		t.Fatalf("retry=%+v calls=%d err=%v", completed, executor.calls, err)
	}
}
