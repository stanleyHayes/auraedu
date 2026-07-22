package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/crm-service/internal/domain"
	"github.com/auraedu/crm-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

type storedRequest struct {
	hash, leadID string
	created      bool
}
type memoryRepo struct {
	leads        map[string]*domain.Lead
	requests     map[string]storedRequest
	interactions []*domain.Interaction
	callbacks    map[string]*domain.CallbackRequest
	callbackKeys map[string]storedCallbackRequest
}

type storedCallbackRequest struct {
	hash, callbackID string
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{leads: map[string]*domain.Lead{}, requests: map[string]storedRequest{}, callbacks: map[string]*domain.CallbackRequest{}, callbackKeys: map[string]storedCallbackRequest{}}
}
func (r *memoryRepo) Capture(_ context.Context, lead *domain.Lead, key, requestHash string, initial *domain.Interaction) (ports.CaptureResult, error) {
	k := lead.TenantID + ":" + key
	if stored, ok := r.requests[k]; ok {
		if stored.hash != requestHash {
			return ports.CaptureResult{}, domain.ErrConflict
		}
		return ports.CaptureResult{Lead: r.leads[stored.leadID], Created: stored.created, Replay: true}, nil
	}
	var found *domain.Lead
	for _, candidate := range r.leads {
		if candidate.TenantID == lead.TenantID && ((lead.Email != nil && candidate.Email != nil && *lead.Email == *candidate.Email) || (lead.Phone != nil && candidate.Phone != nil && *lead.Phone == *candidate.Phone)) {
			found = candidate
			break
		}
	}
	created := found == nil
	if created {
		found = lead
		r.leads[lead.ID] = lead
	}
	if initial != nil {
		initial.LeadID = found.ID
		r.interactions = append(r.interactions, initial)
	}
	r.requests[k] = storedRequest{hash: requestHash, leadID: found.ID, created: created}
	return ports.CaptureResult{Lead: found, Created: created}, nil
}
func (r *memoryRepo) GetLead(_ context.Context, tenantID, id string) (*domain.Lead, error) {
	l := r.leads[id]
	if l == nil || l.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return l, nil
}
func (r *memoryRepo) ListLeads(_ context.Context, tenantID string, _ int, _ string, _ ports.LeadFilter) ([]*domain.Lead, string, error) {
	var out []*domain.Lead
	for _, l := range r.leads {
		if l.TenantID == tenantID {
			out = append(out, l)
		}
	}
	return out, "", nil
}
func (r *memoryRepo) UpdateLead(_ context.Context, tenantID string, l *domain.Lead) error {
	if l.TenantID != tenantID {
		return domain.ErrForbidden
	}
	r.leads[l.ID] = l
	return nil
}
func (r *memoryRepo) CreateInteraction(_ context.Context, tenantID string, i *domain.Interaction) error {
	if i.TenantID != tenantID {
		return domain.ErrForbidden
	}
	r.interactions = append(r.interactions, i)
	return nil
}
func (r *memoryRepo) ListInteractions(_ context.Context, tenantID, leadID string, _ int, _ string) ([]*domain.Interaction, string, error) {
	var out []*domain.Interaction
	for _, i := range r.interactions {
		if i.TenantID == tenantID && i.LeadID == leadID {
			out = append(out, i)
		}
	}
	return out, "", nil
}
func (r *memoryRepo) GetScoringEvidence(_ context.Context, tenantID, leadID string) (domain.ScoringEvidence, error) {
	if _, err := r.GetLead(context.Background(), tenantID, leadID); err != nil {
		return domain.ScoringEvidence{}, err
	}
	var evidence domain.ScoringEvidence
	for _, interaction := range r.interactions {
		if interaction.TenantID == tenantID && interaction.LeadID == leadID && interaction.Direction == "inbound" && interaction.ActorType == "prospect" {
			evidence.InboundProspectInteractions++
			at := interaction.OccurredAt
			if evidence.LastInboundAt == nil || at.After(*evidence.LastInboundAt) {
				evidence.LastInboundAt = &at
			}
		}
	}
	return evidence, nil
}
func (r *memoryRepo) SaveLeadScore(_ context.Context, tenantID, leadID, _ string, score domain.LeadScore) (bool, error) {
	lead, err := r.GetLead(context.Background(), tenantID, leadID)
	if err != nil {
		return false, err
	}
	if lead.Score != nil && *lead.Score == score.Score && lead.ScoreVersion != nil && *lead.ScoreVersion == score.RuleVersion {
		return false, nil
	}
	lead.Score = &score.Score
	lead.ScoreVersion = &score.RuleVersion
	lead.ScoreConfidence = &score.Confidence
	lead.ScorePositiveFactors = score.PositiveFactors
	lead.ScoreNegativeFactors = score.NegativeFactors
	lead.ScoredAt = &score.EvaluatedAt
	return true, nil
}

func (r *memoryRepo) FindCallbackReplay(_ context.Context, tenantID, key, requestHash string) (ports.CallbackResult, bool, error) {
	stored, ok := r.callbackKeys[tenantID+":"+key]
	if !ok {
		return ports.CallbackResult{}, false, nil
	}
	if stored.hash != requestHash {
		return ports.CallbackResult{}, false, domain.ErrConflict
	}
	return ports.CallbackResult{Callback: r.callbacks[stored.callbackID], Replay: true}, true, nil
}

func (r *memoryRepo) ScheduleCallback(_ context.Context, callback *domain.CallbackRequest, key, requestHash string) (ports.CallbackResult, error) {
	if replay, found, err := r.FindCallbackReplay(context.Background(), callback.TenantID, key, requestHash); found || err != nil {
		return replay, err
	}
	r.callbacks[callback.ID] = callback
	r.callbackKeys[callback.TenantID+":"+key] = storedCallbackRequest{hash: requestHash, callbackID: callback.ID}
	return ports.CallbackResult{Callback: callback}, nil
}

func (r *memoryRepo) ListCallbacks(_ context.Context, tenantID string, status domain.CallbackStatus, limit int) ([]*domain.CallbackRequest, error) {
	out := make([]*domain.CallbackRequest, 0, limit)
	for _, callback := range r.callbacks {
		if callback.TenantID == tenantID && (status == "" || callback.Status == status) {
			out = append(out, callback)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

type recordingPublisher struct{ leads, interactions, callbacks, scores int }

func (p *recordingPublisher) LeadCreated(context.Context, *domain.Lead) error { p.leads++; return nil }
func (p *recordingPublisher) InteractionCreated(context.Context, *domain.Interaction) error {
	p.interactions++
	return nil
}
func (p *recordingPublisher) FeedbackSubmitted(context.Context, *domain.Feedback) error { return nil }
func (p *recordingPublisher) CallbackRequested(context.Context, *domain.CallbackRequest) error {
	p.callbacks++
	return nil
}
func (p *recordingPublisher) LeadScored(context.Context, string, string, domain.LeadScore) error {
	p.scores++
	return nil
}

func enabledService() (*Service, *memoryRepo, *recordingPublisher) {
	repo, pub := newMemoryRepo(), &recordingPublisher{}
	gate := flags.NewStaticSnapshot()
	gate.Set("school-a", FeatureGrowthCRM, true)
	gate.Set("school-b", FeatureGrowthCRM, true)
	gate.Set("school-a", FeatureLeadScoring, true)
	gate.Set("school-b", FeatureLeadScoring, true)
	return NewService(repo, WithCallbackRepository(repo), WithFeatureGate(gate), WithPublisher(pub)), repo, pub
}

func captureRequest(email, message string) CaptureRequest {
	return CaptureRequest{FirstName: "Ama", LastName: "Mensah", Email: &email, Source: "website", Message: &message, Consent: domain.Consent{PrivacyNoticeVersion: "2026-01", Email: true}}
}

func TestCaptureCreatesDeduplicatesAndDoesNotRepublishReplay(t *testing.T) {
	svc, repo, pub := enabledService()
	ctx := context.Background()
	req := captureRequest("ama@example.com", "Tell me about nursing")
	first, err := svc.Capture(ctx, "school-a", "key-000000000001", req)
	if err != nil || !first.Created {
		t.Fatalf("first capture=%+v err=%v", first, err)
	}
	replay, err := svc.Capture(ctx, "school-a", "key-000000000001", req)
	if err != nil || !replay.Replay {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	dedupe, err := svc.Capture(ctx, "school-a", "key-000000000002", req)
	if err != nil || dedupe.Created || dedupe.Lead.ID != first.Lead.ID {
		t.Fatalf("dedupe=%+v err=%v", dedupe, err)
	}
	if len(repo.leads) != 1 || len(repo.interactions) != 2 || pub.leads != 1 || pub.interactions != 2 {
		t.Fatalf("leads=%d interactions=%d events=%d/%d", len(repo.leads), len(repo.interactions), pub.leads, pub.interactions)
	}
}

func TestResolveWelcomeRecipientReturnsOnlyConsentedChannels(t *testing.T) {
	svc, _, _ := enabledService()
	email := "prospect@example.com"
	phone := "+233240000123"
	result, err := svc.Capture(context.Background(), "school-a", "recipient-key-0001", CaptureRequest{
		FirstName: "Esi", LastName: "Owusu", Email: &email, Phone: &phone, Source: "website",
		Consent: domain.Consent{PrivacyNoticeVersion: "2026-01", WhatsApp: true, SMS: false, Email: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	recipient, err := svc.ResolveWelcomeRecipient(context.Background(), "school-a", result.Lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recipient.Email != email || recipient.Phone != phone || recipient.FirstName != "Esi" {
		t.Fatalf("contact resolution lost current values: %+v", recipient)
	}
	if recipient.Eligible || recipient.EmailEligible || recipient.SMSEligible || !recipient.WhatsAppEligible {
		t.Fatalf("channel consent was not preserved: %+v", recipient)
	}
}

func TestLeadScoringIsExplainablePermissionedAndReplaySafe(t *testing.T) {
	svc, _, pub := enabledService()
	result, err := svc.Capture(context.Background(), "school-a", "score-00000000001", captureRequest("score@example.com", "Science programmes"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Lead.Score == nil || result.Lead.ScoreVersion == nil || len(result.Lead.ScorePositiveFactors) == 0 || pub.scores != 1 {
		t.Fatalf("score=%+v events=%d", result.Lead, pub.scores)
	}
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: "school-a"})
	viewer := auth.Actor{UserID: "viewer", TenantID: "school-a", Permissions: []string{PermRead}}
	if _, err := svc.RescoreLead(ctx, viewer, result.Lead.ID); err == nil {
		t.Fatal("viewer must not rescore")
	}
	editor := auth.Actor{UserID: "editor", TenantID: "school-a", Permissions: []string{PermUpdate}}
	if _, err := svc.RescoreLead(ctx, editor, result.Lead.ID); err != nil {
		t.Fatal(err)
	}
	if pub.scores != 1 {
		t.Fatalf("unchanged rescore emitted duplicate event: %d", pub.scores)
	}
}

func TestCapturePreservesExplicitEmptyProgrammeList(t *testing.T) {
	svc, _, _ := enabledService()
	request := captureRequest("empty-programmes@example.com", "Hello")
	request.ProgrammeIDs = []string{}
	result, err := svc.Capture(context.Background(), "school-a", "key-empty-programmes", request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Lead.PreferredProgrammeIDs == nil || len(result.Lead.PreferredProgrammeIDs) != 0 {
		t.Fatalf("expected non-nil empty programme list, got %#v", result.Lead.PreferredProgrammeIDs)
	}
}

func TestCaptureIsTenantLocalAndIdempotencyDetectsConflict(t *testing.T) {
	svc, repo, _ := enabledService()
	req := captureRequest("ama@example.com", "Hello")
	if _, err := svc.Capture(context.Background(), "school-a", "key-000000000001", req); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Capture(context.Background(), "school-b", "key-000000000001", req); err != nil {
		t.Fatal(err)
	}
	changed := captureRequest("different@example.com", "Hello")
	if _, err := svc.Capture(context.Background(), "school-a", "key-000000000001", changed); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	if len(repo.leads) != 2 {
		t.Fatalf("expected one lead per tenant, got %d", len(repo.leads))
	}
}

func TestStaffReadsRequireTenantPermissionAndFeature(t *testing.T) {
	svc, _, _ := enabledService()
	result, err := svc.Capture(context.Background(), "school-a", "key-000000000001", captureRequest("ama@example.com", "Hello"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: "school-a"})
	actor := auth.Actor{UserID: "officer", TenantID: "school-a", Permissions: []string{PermRead}}
	if _, err := svc.GetLead(ctx, actor, result.Lead.ID); err != nil {
		t.Fatalf("authorized read: %v", err)
	}
	other := auth.Actor{UserID: "officer", TenantID: "school-b", Permissions: []string{PermRead}}
	if _, err := svc.GetLead(ctx, other, result.Lead.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestScheduleCallbackRequiresVoiceConsentAndIsReplaySafe(t *testing.T) {
	svc, repo, pub := enabledService()
	phone, message := "+233240000000", "Please call about admissions"
	request := ScheduleCallbackRequest{
		FirstName: "Ama", LastName: "Mensah", Phone: &phone, PreferredAt: time.Now().Add(2 * time.Hour),
		Timezone: "Africa/Accra", Locale: "en-GH", Message: &message,
		Consent: domain.Consent{PrivacyNoticeVersion: "2026-01", Voice: true},
	}
	first, err := svc.ScheduleCallback(context.Background(), "school-a", "callback-00000001", request)
	if err != nil || first.Replay || first.Callback == nil {
		t.Fatalf("first callback=%+v err=%v", first, err)
	}
	replay, err := svc.ScheduleCallback(context.Background(), "school-a", "callback-00000001", request)
	if err != nil || !replay.Replay || replay.Callback.ID != first.Callback.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	changed := request
	changed.Locale = "fr-GH"
	if _, err := svc.ScheduleCallback(context.Background(), "school-a", "callback-00000001", changed); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	request.Consent.Voice = false
	if _, err := svc.ScheduleCallback(context.Background(), "school-a", "callback-00000002", request); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected voice consent validation, got %v", err)
	}
	if len(repo.leads) != 1 || len(repo.callbacks) != 1 || pub.callbacks != 1 {
		t.Fatalf("leads=%d callbacks=%d events=%d", len(repo.leads), len(repo.callbacks), pub.callbacks)
	}
	if firstLead := repo.leads[first.Callback.LeadID]; firstLead == nil || firstLead.PreferredProgrammeIDs == nil {
		t.Fatalf("callback capture must preserve an empty programme list: %+v", firstLead)
	}
}

func TestScheduleCallbackReplaySurvivesPreferredTimeWindowPassing(t *testing.T) {
	svc, repo, _ := enabledService()
	phone, message := "+233240000001", "Call me"
	request := ScheduleCallbackRequest{FirstName: "Kojo", LastName: "Owusu", Phone: &phone, PreferredAt: time.Now().Add(2 * time.Hour), Timezone: "Africa/Accra", Locale: "en", Message: &message, Consent: domain.Consent{PrivacyNoticeVersion: "2026-01", Voice: true}}
	first, err := svc.ScheduleCallback(context.Background(), "school-a", "callback-00000003", request)
	if err != nil {
		t.Fatal(err)
	}
	first.Callback.PreferredAt = time.Now().Add(-time.Hour)
	repo.callbacks[first.Callback.ID] = first.Callback
	replay, err := svc.ScheduleCallback(context.Background(), "school-a", "callback-00000003", request)
	if err != nil || !replay.Replay || replay.Callback.ID != first.Callback.ID {
		t.Fatalf("historical replay=%+v err=%v", replay, err)
	}
}

func TestListCallbacksRequiresTenantPermission(t *testing.T) {
	svc, _, _ := enabledService()
	phone, message := "+233240000002", "Call"
	_, err := svc.ScheduleCallback(context.Background(), "school-a", "callback-00000004", ScheduleCallbackRequest{FirstName: "Esi", LastName: "Quaye", Phone: &phone, PreferredAt: time.Now().Add(time.Hour), Timezone: "Africa/Accra", Locale: "en-GH", Message: &message, Consent: domain.Consent{PrivacyNoticeVersion: "2026-01", Voice: true}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: "school-a"})
	actor := auth.Actor{UserID: "officer", TenantID: "school-a", Permissions: []string{PermRead}}
	items, err := svc.ListCallbacks(ctx, actor, domain.CallbackRequested, 25)
	if err != nil || len(items) != 1 {
		t.Fatalf("authorized list=%+v err=%v", items, err)
	}
	actor.TenantID = "school-b"
	if _, err := svc.ListCallbacks(ctx, actor, "", 25); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
