package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const defaultDependencyProbeTimeout = 3 * time.Second

// Dependency is one private service whose readiness is visible to platform
// operators. URL is never returned to the caller.
type Dependency struct {
	Service string
	URL     string
	Path    string
}

type DependencyCheck struct {
	Service   string `json:"service"`
	Endpoint  string `json:"endpoint"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
	LatencyMS int64  `json:"latency_ms"`
}

type DependencyHealthReport struct {
	Status      string            `json:"status"`
	GeneratedAt string            `json:"generated_at"`
	Checks      []DependencyCheck `json:"checks"`
}

// DependencyHealthHandler performs bounded, concurrent readiness probes. It
// deliberately exposes only service names, health paths and safe status detail;
// private service hosts and transport errors stay inside the gateway boundary.
type DependencyHealthHandler struct {
	dependencies []Dependency
	client       *http.Client
	timeout      time.Duration
}

func NewDependencyHealthHandler(dependencies []Dependency, client *http.Client, timeout time.Duration) *DependencyHealthHandler {
	if client == nil {
		client = &http.Client{Transport: &http.Transport{
			MaxIdleConns:        64,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     30 * time.Second,
		}, CheckRedirect: func(*http.Request, []*http.Request) error {
			// A readiness probe must never follow an upstream redirect outside the
			// configured private service origin.
			return http.ErrUseLastResponse
		}}
	}
	if timeout <= 0 {
		timeout = defaultDependencyProbeTimeout
	}
	copyOfDependencies := append([]Dependency(nil), dependencies...)
	sort.Slice(copyOfDependencies, func(i, j int) bool {
		return copyOfDependencies[i].Service < copyOfDependencies[j].Service
	})
	return &DependencyHealthHandler{dependencies: copyOfDependencies, client: client, timeout: timeout}
}

func (h *DependencyHealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET is supported")
		return
	}
	actor := ActorFrom(r.Context())
	if actor.IsEmpty() || !actor.Platform {
		writeJSONError(w, http.StatusForbidden, "platform_admin_required", "platform administrator access is required")
		return
	}

	checks := make([]DependencyCheck, len(h.dependencies))
	var wg sync.WaitGroup
	for index, dependency := range h.dependencies {
		wg.Add(1)
		go func() {
			defer wg.Done()
			checks[index] = h.probe(r.Context(), dependency)
		}()
	}
	wg.Wait()

	report := DependencyHealthReport{
		Status:      aggregateDependencyStatus(checks),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Checks:      checks,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(report)
}

func (h *DependencyHealthHandler) probe(parent context.Context, dependency Dependency) DependencyCheck {
	path := dependency.Path
	if path == "" {
		path = "/ready"
	}
	result := DependencyCheck{Service: dependency.Service, Endpoint: path, Status: "unreachable", Detail: "connection failed"}
	started := time.Now()
	ctx, cancel := context.WithTimeout(parent, h.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(dependency.URL, "/")+path, nil)
	if err != nil {
		result.Detail = "invalid probe configuration"
		return result
	}
	response, err := h.client.Do(req)
	result.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		if ctx.Err() != nil {
			result.Detail = "timeout"
		}
		return result
	}
	defer response.Body.Close() //nolint:errcheck // Close errors cannot affect a completed health probe.
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		result.Status = "healthy"
		result.Detail = "ready"
		return result
	}
	result.Status = "degraded"
	result.Detail = http.StatusText(response.StatusCode)
	if result.Detail == "" {
		result.Detail = "non-success response"
	}
	return result
}

func aggregateDependencyStatus(checks []DependencyCheck) string {
	if len(checks) == 0 {
		return "down"
	}
	healthy := 0
	for _, check := range checks {
		if check.Status == "healthy" {
			healthy++
		}
	}
	if healthy == len(checks) {
		return "healthy"
	}
	if healthy == 0 {
		allUnreachable := true
		for _, check := range checks {
			if check.Status != "unreachable" {
				allUnreachable = false
				break
			}
		}
		if allUnreachable {
			return "down"
		}
	}
	return "degraded"
}

// DefaultDependencies is the private service readiness inventory owned by the
// gateway. Every service exposes /ready with its PostgreSQL check.
func DefaultDependencies() []Dependency {
	return []Dependency{
		{Service: "academic-service", URL: envURL("SERVICE_ACADEMIC_URL", "http://localhost:8092"), Path: "/ready"},
		{Service: "admissions-service", URL: envURL("SERVICE_ADMISSIONS_URL", "http://localhost:8114"), Path: "/ready"},
		{Service: "ai-orchestrator-service", URL: envURL("SERVICE_AI_ORCHESTRATOR_URL", "http://localhost:8111"), Path: "/ready"},
		{Service: "ai-prediction-service", URL: envURL("SERVICE_AI_PREDICTION_URL", "http://localhost:8201"), Path: "/ready"},
		{Service: "ai-recommendation-service", URL: envURL("SERVICE_AI_RECOMMENDATION_URL", "http://localhost:8200"), Path: "/ready"},
		{Service: "analytics-service", URL: envURL("SERVICE_ANALYTICS_URL", "http://localhost:8102"), Path: "/ready"},
		{Service: "assessment-service", URL: envURL("SERVICE_ASSESSMENT_URL", "http://localhost:8095"), Path: "/ready"},
		{Service: "attendance-service", URL: envURL("SERVICE_ATTENDANCE_URL", "http://localhost:8094"), Path: "/ready"},
		{Service: "audit-service", URL: envURL("SERVICE_AUDIT_URL", "http://localhost:8104"), Path: "/ready"},
		{Service: "billing-service", URL: envURL("SERVICE_BILLING_URL", "http://localhost:8100"), Path: "/ready"},
		{Service: "campaign-service", URL: envURL("SERVICE_CAMPAIGN_URL", "http://localhost:8113"), Path: "/ready"},
		{Service: "career-guidance-service", URL: envURL("SERVICE_CAREER_GUIDANCE_URL", "http://localhost:8112"), Path: "/ready"},
		{Service: "cbt-service", URL: envURL("SERVICE_CBT_URL", "http://localhost:8103"), Path: "/ready"},
		{Service: "crm-service", URL: envURL("SERVICE_CRM_URL", "http://localhost:8105"), Path: "/ready"},
		{Service: "fees-service", URL: envURL("SERVICE_FEES_URL", "http://localhost:8097"), Path: "/ready"},
		{Service: "file-service", URL: envURL("SERVICE_FILE_URL", "http://localhost:8093"), Path: "/ready"},
		{Service: "identity-service", URL: envURL("SERVICE_IDENTITY_URL", "http://localhost:8081"), Path: "/ready"},
		{Service: "knowledge-service", URL: envURL("SERVICE_KNOWLEDGE_URL", "http://localhost:8110"), Path: "/ready"},
		{Service: "market-intelligence-service", URL: envURL("SERVICE_MARKET_INTELLIGENCE_URL", "http://localhost:8115"), Path: "/ready"},
		{Service: "notification-service", URL: envURL("SERVICE_NOTIFICATION_URL", "http://localhost:8099"), Path: "/ready"},
		{Service: "payment-service", URL: envURL("SERVICE_PAYMENT_URL", "http://localhost:8098"), Path: "/ready"},
		{Service: "report-service", URL: envURL("SERVICE_REPORT_URL", "http://localhost:8096"), Path: "/ready"},
		{Service: "staff-service", URL: envURL("SERVICE_STAFF_URL", "http://localhost:8091"), Path: "/ready"},
		{Service: "student-service", URL: envURL("SERVICE_STUDENT_URL", "http://localhost:8090"), Path: "/ready"},
		{Service: "tenant-service", URL: envURL("SERVICE_TENANT_URL", "http://localhost:8082"), Path: "/ready"},
		{Service: "website-service", URL: envURL("SERVICE_WEBSITE_URL", "http://localhost:8101"), Path: "/ready"},
	}
}
