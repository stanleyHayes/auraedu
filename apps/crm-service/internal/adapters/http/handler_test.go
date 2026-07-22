package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/auraedu/crm-service/internal/application"
	"github.com/auraedu/crm-service/internal/domain"
	"github.com/auraedu/crm-service/internal/ports"
	"github.com/auraedu/platform/flags"
)

type captureRepo struct {
	captured                  *domain.Lead
	feedback                  *domain.Feedback
	callback                  *domain.CallbackRequest
	callbackKey, callbackHash string
}

func (r *captureRepo) Capture(_ context.Context, lead *domain.Lead, _, _ string, _ *domain.Interaction) (ports.CaptureResult, error) {
	r.captured = lead
	return ports.CaptureResult{Lead: lead, Created: true}, nil
}
func (r *captureRepo) GetLead(_ context.Context, tenantID, id string) (*domain.Lead, error) {
	if r.captured != nil && r.captured.TenantID == tenantID && r.captured.ID == id {
		return r.captured, nil
	}
	return nil, domain.ErrNotFound
}
func (*captureRepo) ListLeads(context.Context, string, int, string, ports.LeadFilter) ([]*domain.Lead, string, error) {
	return nil, "", nil
}
func (*captureRepo) UpdateLead(context.Context, string, *domain.Lead) error               { return nil }
func (*captureRepo) CreateInteraction(context.Context, string, *domain.Interaction) error { return nil }
func (*captureRepo) ListInteractions(context.Context, string, string, int, string) ([]*domain.Interaction, string, error) {
	return nil, "", nil
}
func (*captureRepo) GetScoringEvidence(context.Context, string, string) (domain.ScoringEvidence, error) {
	return domain.ScoringEvidence{}, nil
}
func (*captureRepo) SaveLeadScore(context.Context, string, string, string, domain.LeadScore) (bool, error) {
	return true, nil
}
func (r *captureRepo) SubmitFeedback(_ context.Context, feedback *domain.Feedback, _, _ string) (ports.FeedbackResult, error) {
	r.feedback = feedback
	return ports.FeedbackResult{Feedback: feedback}, nil
}
func (r *captureRepo) FindCallbackReplay(_ context.Context, _ string, key, requestHash string) (ports.CallbackResult, bool, error) {
	if r.callback == nil || r.callbackKey != key {
		return ports.CallbackResult{}, false, nil
	}
	if r.callbackHash != requestHash {
		return ports.CallbackResult{}, false, domain.ErrConflict
	}
	return ports.CallbackResult{Callback: r.callback, Replay: true}, true, nil
}
func (r *captureRepo) ScheduleCallback(_ context.Context, callback *domain.CallbackRequest, key string, requestHash string) (ports.CallbackResult, error) {
	r.callback, r.callbackKey, r.callbackHash = callback, key, requestHash
	return ports.CallbackResult{Callback: callback}, nil
}
func (r *captureRepo) ListCallbacks(_ context.Context, tenantID string, status domain.CallbackStatus, _ int) ([]*domain.CallbackRequest, error) {
	if r.callback != nil && r.callback.TenantID == tenantID && (status == "" || r.callback.Status == status) {
		return []*domain.CallbackRequest{r.callback}, nil
	}
	return nil, nil
}

func captureMux() (*http.ServeMux, *captureRepo) {
	repo := &captureRepo{}
	gate := flags.NewStaticSnapshot()
	gate.Set("school-a", application.FeatureGrowthCRM, true)
	gate.Set("school-a", application.FeatureLeadScoring, true)
	mux := http.NewServeMux()
	NewHandler(application.NewService(repo, application.WithFeedbackRepository(repo), application.WithCallbackRepository(repo), application.WithFeatureGate(gate))).Register(mux)
	return mux, repo
}

func TestRescoreLeadReturnsExplanation(t *testing.T) {
	mux, repo := captureMux()
	lead, err := domain.NewLead("school-a", "Ama", "Mensah", ptr("score@example.com"), nil, "website", domain.Consent{PrivacyNoticeVersion: "2026-01"})
	if err != nil {
		t.Fatal(err)
	}
	repo.captured = lead
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/leads/"+lead.ID+"/score", nil)
	request.Header.Set("X-Tenant-ID", "school-a")
	request.Header.Set("X-Actor-Tenant", "school-a")
	request.Header.Set("X-Actor-User", "editor")
	request.Header.Set("X-Actor-Permissions", application.PermUpdate)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"rule_version":"growth-rules-2026-01"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func ptr(value string) *string { return &value }

func TestSubmitFeedbackMatchesContract(t *testing.T) {
	mux, repo := captureMux()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/public/feedback", strings.NewReader(`{"feedback_type":"helpful","rating":5,"comment":"Clear answer"}`))
	request.Header.Set("X-Tenant-Code", "school-a")
	request.Header.Set("Idempotency-Key", "feedback-000000001")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if repo.feedback == nil || repo.feedback.TenantID != "school-a" || repo.feedback.ReviewStatus != "pending" {
		t.Fatalf("unexpected feedback: %+v", repo.feedback)
	}
}

func TestCaptureLeadMatchesContract(t *testing.T) {
	mux, repo := captureMux()
	body := `{"first_name":"Ama","last_name":"Mensah","email":"ama@example.com","source":"website","consent":{"privacy_notice_version":"2026-01","email":true}}`
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/public/leads", strings.NewReader(body))
	request.Header.Set("X-Tenant-Code", "school-a")
	request.Header.Set("Idempotency-Key", "capture-0000000001")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if repo.captured == nil || repo.captured.TenantID != "school-a" || repo.captured.Email == nil || *repo.captured.Email != "ama@example.com" {
		t.Fatalf("unexpected captured lead: %+v", repo.captured)
	}
	if !strings.Contains(response.Body.String(), `"created":true`) || !strings.Contains(response.Body.String(), `"stage":"new"`) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}

func TestCaptureLeadRequiresIdempotencyKey(t *testing.T) {
	mux, _ := captureMux()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/public/leads", strings.NewReader(`{}`))
	request.Header.Set("X-Tenant-Code", "school-a")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", response.Code)
	}
}

func TestScheduleCallbackMatchesContractAndReplays(t *testing.T) {
	mux, repo := captureMux()
	preferred := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	body := `{"first_name":"Ama","last_name":"Mensah","phone":"+233240000000","preferred_at":"` + preferred + `","timezone":"Africa/Accra","locale":"en-GH","message":"Please call me","consent":{"privacy_notice_version":"2026-01","voice":true}}`
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/public/callback-requests", strings.NewReader(body))
	request.Header.Set("X-Tenant-Code", "school-a")
	request.Header.Set("Idempotency-Key", "callback-00000001")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if repo.callback == nil || repo.callback.TenantID != "school-a" || repo.callback.Timezone != "Africa/Accra" {
		t.Fatalf("unexpected callback: %+v", repo.callback)
	}
	replayRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/public/callback-requests", strings.NewReader(body))
	replayRequest.Header = request.Header.Clone()
	replayResponse := httptest.NewRecorder()
	mux.ServeHTTP(replayResponse, replayRequest)
	if replayResponse.Code != http.StatusOK || !strings.Contains(replayResponse.Body.String(), repo.callback.ID) {
		t.Fatalf("replay status=%d body=%s", replayResponse.Code, replayResponse.Body.String())
	}
}

func TestScheduleCallbackRequiresVoiceConsent(t *testing.T) {
	mux, _ := captureMux()
	preferred := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	body := `{"first_name":"Ama","last_name":"Mensah","phone":"+233240000000","preferred_at":"` + preferred + `","timezone":"Africa/Accra","locale":"en","message":"Call","consent":{"privacy_notice_version":"2026-01","voice":false}}`
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/public/callback-requests", strings.NewReader(body))
	request.Header.Set("X-Tenant-Code", "school-a")
	request.Header.Set("Idempotency-Key", "callback-00000002")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", response.Code, response.Body.String())
	}
}
