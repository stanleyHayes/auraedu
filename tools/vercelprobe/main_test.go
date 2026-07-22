package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type fixture struct {
	missingMarketingGatewayEnv bool
	staleWebSHA                bool
	rejectMarketingCORS        bool
}

func testConfig() Config {
	return Config{
		Name: canonicalName, Environment: "production", VercelAPIBase: canonicalAPIBase,
		RequestTimeout: duration{2 * time.Second}, MaxResponseBytes: 1 << 20,
		RunID: "release-vercel-test", GitSHA: "abcdef1234567890", Token: "test-" + strings.Repeat("x", 24),
		TeamID: "team_auraedu", WebProject: "portal-project", MarketingProject: "marketing-project",
		WebURL: "https://portal-auraedu.vercel.app", MarketingURL: "https://marketing-auraedu.vercel.app",
		GatewayURL: "https://auraedu-gateway.onrender.com",
	}
}

func testClient(t *testing.T, cfg Config, state fixture) *http.Client {
	t.Helper()
	return &http.Client{
		Timeout: cfg.RequestTimeout.Duration,
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Host == "api.vercel.com" {
				if request.Header.Get("Authorization") != "Bearer "+cfg.Token || request.URL.Query().Get("teamId") != cfg.TeamID {
					return response(request, http.StatusUnauthorized, `{}`, nil), nil
				}
				return vercelResponse(request, cfg, state), nil
			}
			switch request.URL.Host {
			case "portal-auraedu.vercel.app":
				return applicationResponse(request, "<html><body>Welcome back</body></html>"), nil
			case "marketing-auraedu.vercel.app":
				return applicationResponse(request, "<html><body>Run your school clearly</body></html>"), nil
			case "auraedu-gateway.onrender.com":
				origin := request.Header.Get("Origin")
				headers := http.Header{
					"Access-Control-Allow-Origin":  {origin},
					"Access-Control-Allow-Methods": {"GET, POST, OPTIONS"},
					"Vary":                         {"Origin"},
				}
				if state.rejectMarketingCORS && origin == cfg.MarketingURL {
					headers.Del("Access-Control-Allow-Origin")
				}
				return response(request, http.StatusNoContent, "", headers), nil
			default:
				return response(request, http.StatusNotFound, "", nil), nil
			}
		}),
	}
}

func vercelResponse(request *http.Request, cfg Config, state fixture) *http.Response {
	switch request.URL.Path {
	case "/v9/projects/portal-project":
		return response(request, http.StatusOK,
			`{"id":"prj_portal_secret","name":"portal-project","framework":"nextjs","rootDirectory":"apps/web","link":{"type":"github","repo":"auraedu"}}`, nil)
	case "/v9/projects/marketing-project":
		return response(request, http.StatusOK,
			`{"id":"prj_marketing_secret","name":"marketing-project","framework":"nextjs","rootDirectory":"apps/marketing","link":{"type":"github","repo":"auraedu"}}`, nil)
	case "/v10/projects/portal-project/env":
		return response(request, http.StatusOK, environmentJSON(false), nil)
	case "/v10/projects/marketing-project/env":
		return response(request, http.StatusOK, environmentJSON(state.missingMarketingGatewayEnv), nil)
	case "/v7/deployments":
		projectID := request.URL.Query().Get("projectId")
		sha := cfg.GitSHA
		if state.staleWebSHA && projectID == "prj_portal_secret" {
			sha = "1111111111111111"
		}
		body := `{"deployments":[{"uid":"dpl_release_secret","projectId":"` + projectID +
			`","url":"release-abc.vercel.app","state":"READY","readyState":"READY","target":"production","meta":{"githubCommitSha":"` + sha + `"}}]}`
		return response(request, http.StatusOK, body, nil)
	default:
		return response(request, http.StatusNotFound, `{}`, nil)
	}
}

func environmentJSON(withoutGateway bool) string {
	entries := []string{
		`{"key":"ENVIRONMENT","target":["production"]}`,
		`{"key":"NEXT_PUBLIC_API_URL","target":["production"]}`,
		`{"key":"NEXT_PUBLIC_APP_URL","target":["production"]}`,
	}
	if !withoutGateway {
		entries = append(entries, `{"key":"AURAEDU_API_URL","target":["production"]}`)
	}
	return "[" + strings.Join(entries, ",") + "]"
}

func applicationResponse(request *http.Request, body string) *http.Response {
	headers := http.Header{
		"Strict-Transport-Security": {"max-age=63072000; includeSubDomains; preload"},
		"X-Content-Type-Options":    {"nosniff"},
		"X-Frame-Options":           {"DENY"},
		"Referrer-Policy":           {"strict-origin-when-cross-origin"},
		"Permissions-Policy":        {"camera=(), microphone=(), geolocation=()"},
	}
	return response(request, http.StatusOK, body, headers)
}

func response(request *http.Request, status int, body string, headers http.Header) *http.Response {
	if headers == nil {
		headers = make(http.Header)
	}
	return &http.Response{
		StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body)), Request: request,
	}
}

func TestRunProvesTwoProductionFrontendsWithoutRetainingSecrets(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	evidence, err := run(context.Background(), cfg, testClient(t, cfg, fixture{}))
	if err != nil || !evidence.AllPassed || len(evidence.Checks) != 6 {
		t.Fatalf("evidence=%+v err=%v", evidence, err)
	}
	wantNames := []string{
		"web-project-linked", "marketing-project-linked", "environment-configured",
		"web-production-deployed", "marketing-production-deployed", "gateway-cors-observed",
	}
	for index, check := range evidence.Checks {
		if check.Name != wantNames[index] || !check.Passed || len(check.EvidenceFingerprint) != 16 {
			t.Fatalf("check[%d]=%+v", index, check)
		}
	}
	encoded, marshalErr := json.Marshal(evidence)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, secret := range []string{cfg.Token, cfg.TeamID, "prj_portal_secret", "prj_marketing_secret", "dpl_release_secret"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("evidence leaked %q", secret)
		}
	}
}

func TestRunRejectsIncompleteProductionEnvironment(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	evidence, err := run(context.Background(), cfg, testClient(t, cfg, fixture{missingMarketingGatewayEnv: true}))
	if err == nil || evidence.AllPassed || evidence.Checks[2].Passed {
		t.Fatalf("incomplete environment accepted: evidence=%+v err=%v", evidence, err)
	}
}

func TestRunRejectsDeploymentFromAnotherGitRevision(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	evidence, err := run(context.Background(), cfg, testClient(t, cfg, fixture{staleWebSHA: true}))
	if err == nil || evidence.AllPassed || evidence.Checks[3].Passed {
		t.Fatalf("stale deployment accepted: evidence=%+v err=%v", evidence, err)
	}
}

func TestRunRequiresBothOriginsAtGatewayBoundary(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	evidence, err := run(context.Background(), cfg, testClient(t, cfg, fixture{rejectMarketingCORS: true}))
	if err == nil || evidence.AllPassed || evidence.Checks[5].Passed {
		t.Fatalf("partial CORS accepted: evidence=%+v err=%v", evidence, err)
	}
}

func TestDeploymentQueryPinsProductionReadyGitSHA(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	var observed url.Values
	client := testClient(t, cfg, fixture{})
	base := client.Transport
	client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/v7/deployments" {
			observed = request.URL.Query()
		}
		return base.RoundTrip(request)
	})
	if _, err := run(context.Background(), cfg, client); err != nil {
		t.Fatal(err)
	}
	if observed.Get("target") != "production" || observed.Get("state") != "READY" ||
		observed.Get("sha") != cfg.GitSHA || observed.Get("limit") != "1" {
		t.Fatalf("deployment query=%v", observed)
	}
}

func TestExecutionRejectsOneOriginForTwoProjectsAndPlaceholders(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	if err := validate(cfg); err != nil {
		t.Fatal(err)
	}
	if err := validateExecution(cfg); err != nil {
		t.Fatal(err)
	}
	cfg.MarketingURL = cfg.WebURL
	if err := validateExecution(cfg); err == nil {
		t.Fatal("one public origin accepted for two projects")
	}
	cfg = testConfig()
	cfg.GatewayURL = "https://gateway.auraedu.example"
	if err := validateExecution(cfg); err == nil {
		t.Fatal("placeholder gateway accepted")
	}
	cfg = testConfig()
	cfg.Token = "replace-at-runtime-token"
	if err := validateExecution(cfg); err == nil {
		t.Fatal("placeholder token accepted")
	}
}

func TestEnvironmentDecoderAcceptsWrappedAndBareResponses(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		`[{"key":"ENVIRONMENT","target":["production"]}]`,
		`{"envs":[{"key":"ENVIRONMENT","target":"production"}]}`,
	} {
		entries, err := decodeEnvironment(json.RawMessage(raw))
		if err != nil || len(entries) != 1 || !targetIncludesProduction(entries[0].Target) {
			t.Fatalf("raw=%s entries=%+v err=%v", raw, entries, err)
		}
	}
}

func TestWriteEvidenceUsesExclusiveCreation(t *testing.T) {
	t.Parallel()
	path := t.TempDir() + "/evidence.json"
	if err := writeEvidence(path, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if err := writeEvidence(path, []byte("replacement\n")); err == nil {
		t.Fatal("evidence file was overwritten")
	}
}
