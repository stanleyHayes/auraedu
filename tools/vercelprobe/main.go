// Command vercelprobe proves that the two AuraEDU frontends and their API boundary are production-ready.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	canonicalName    = "auraedu-production-vercel-frontends"
	canonicalAPIBase = "https://api.vercel.com"
)

type duration struct{ time.Duration }

func (d *duration) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

type Config struct {
	Name             string   `json:"name"`
	Environment      string   `json:"environment"`
	VercelAPIBase    string   `json:"vercel_api_base"`
	RequestTimeout   duration `json:"request_timeout"`
	MaxResponseBytes int64    `json:"max_response_bytes"`
	RunID            string   `json:"-"`
	GitSHA           string   `json:"-"`
	Token            string   `json:"-"`
	TeamID           string   `json:"-"`
	WebProject       string   `json:"-"`
	MarketingProject string   `json:"-"`
	WebURL           string   `json:"-"`
	MarketingURL     string   `json:"-"`
	GatewayURL       string   `json:"-"`
}

type EvidenceCheck struct {
	Name                string    `json:"name"`
	Passed              bool      `json:"passed"`
	ObservedAt          time.Time `json:"observed_at"`
	EvidenceFingerprint string    `json:"evidence_fingerprint"`
}

type Evidence struct {
	Name        string          `json:"name"`
	Environment string          `json:"environment"`
	TargetURL   string          `json:"target_url"`
	RunID       string          `json:"run_id"`
	GitSHA      string          `json:"git_sha"`
	StartedAt   time.Time       `json:"started_at"`
	FinishedAt  time.Time       `json:"finished_at"`
	AllPassed   bool            `json:"all_passed"`
	Checks      []EvidenceCheck `json:"checks"`
}

type projectRecord struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Framework     string          `json:"framework"`
	RootDirectory string          `json:"rootDirectory"`
	Link          json.RawMessage `json:"link"`
}

type environmentVariable struct {
	Key    string          `json:"key"`
	Target json.RawMessage `json:"target"`
}

type deploymentRecord struct {
	UID        string            `json:"uid"`
	ProjectID  string            `json:"projectId"`
	URL        string            `json:"url"`
	State      string            `json:"state"`
	ReadyState string            `json:"readyState"`
	Target     string            `json:"target"`
	Meta       map[string]string `json:"meta"`
}

type checkOutcome struct {
	passed   bool
	material string
	failure  string
}

var (
	runIDPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{7,127}$`)
	gitSHAPattern  = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
	projectPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)
)

func main() {
	configPath := flag.String("config", "", "Vercel production scenario JSON file")
	resultPath := flag.String("out", "", "exclusive JSON evidence path")
	validateOnly := flag.Bool("validate-only", false, "validate the versioned scenario without calling Vercel")
	flag.Parse()
	if *configPath == "" {
		fatal(2, "-config is required")
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fatal(2, err.Error())
	}
	if *validateOnly {
		fmt.Printf("Vercel scenario %q valid for %s\n", cfg.Name, cfg.Environment)
		return
	}
	if *resultPath == "" {
		fatal(2, "-out is required for a production Vercel run")
	}
	if err := validateExecution(cfg); err != nil {
		fatal(2, err.Error())
	}
	client := &http.Client{Timeout: cfg.RequestTimeout.Duration}
	evidence, runErr := run(context.Background(), cfg, client)
	encoded, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		fatal(1, "encode evidence: "+err.Error())
	}
	if err := writeEvidence(*resultPath, append(encoded, '\n')); err != nil {
		fatal(1, "write evidence: "+err.Error())
	}
	fmt.Println(string(encoded))
	if runErr != nil {
		fatal(1, runErr.Error())
	}
}

func fatal(code int, message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(code)
}

func loadConfig(path string) (Config, error) {
	// #nosec G304 -- the operator explicitly selects this local deployment scenario file.
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read scenario: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse scenario: %w", err)
	}
	cfg.RunID = env("AURA_VERCEL_RUN_ID")
	cfg.GitSHA = env("AURA_VERCEL_GIT_SHA")
	cfg.Token = env("AURA_VERCEL_TOKEN")
	cfg.TeamID = env("AURA_VERCEL_TEAM_ID")
	cfg.WebProject = env("AURA_VERCEL_WEB_PROJECT")
	cfg.MarketingProject = env("AURA_VERCEL_MARKETING_PROJECT")
	cfg.WebURL = env("AURA_VERCEL_WEB_URL")
	cfg.MarketingURL = env("AURA_VERCEL_MARKETING_URL")
	cfg.GatewayURL = env("AURA_VERCEL_GATEWAY_URL")
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func env(key string) string { return strings.TrimSpace(os.Getenv(key)) }

func validate(cfg Config) error {
	if cfg.Name != canonicalName || cfg.Environment != "production" {
		return errors.New("canonical name and production environment are required")
	}
	if cfg.VercelAPIBase != canonicalAPIBase {
		return errors.New("vercel_api_base must use the canonical Vercel API origin")
	}
	if cfg.RequestTimeout.Duration < time.Second || cfg.RequestTimeout.Duration > 30*time.Second {
		return errors.New("request_timeout must be between one and thirty seconds")
	}
	if cfg.MaxResponseBytes < 32<<10 || cfg.MaxResponseBytes > 2<<20 {
		return errors.New("max_response_bytes must be between 32768 and 2097152")
	}
	return nil
}

func validateExecution(cfg Config) error {
	if !runIDPattern.MatchString(cfg.RunID) || !gitSHAPattern.MatchString(cfg.GitSHA) {
		return errors.New("AURA_VERCEL_RUN_ID and AURA_VERCEL_GIT_SHA are required")
	}
	if len(cfg.Token) < 20 || strings.Contains(strings.ToLower(cfg.Token), "replace") {
		return errors.New("AURA_VERCEL_TOKEN is required and cannot be a placeholder")
	}
	if !projectPattern.MatchString(cfg.WebProject) || !projectPattern.MatchString(cfg.MarketingProject) || cfg.WebProject == cfg.MarketingProject {
		return errors.New("distinct Vercel web and marketing project IDs or names are required")
	}
	if cfg.TeamID != "" && !projectPattern.MatchString(cfg.TeamID) {
		return errors.New("AURA_VERCEL_TEAM_ID is invalid")
	}
	web, err := productionOrigin(cfg.WebURL)
	if err != nil {
		return fmt.Errorf("AURA_VERCEL_WEB_URL: %w", err)
	}
	marketing, err := productionOrigin(cfg.MarketingURL)
	if err != nil {
		return fmt.Errorf("AURA_VERCEL_MARKETING_URL: %w", err)
	}
	if web.String() == marketing.String() {
		return errors.New("web and marketing must use distinct public origins")
	}
	if _, err := productionOrigin(cfg.GatewayURL); err != nil {
		return fmt.Errorf("AURA_VERCEL_GATEWAY_URL: %w", err)
	}
	return nil
}

func productionOrigin(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.Port() != "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, errors.New("must be a credential-free HTTPS origin")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasSuffix(host, ".example") {
		return nil, errors.New("cannot be loopback or a placeholder")
	}
	parsed.Path = ""
	return parsed, nil
}

func run(ctx context.Context, cfg Config, client *http.Client) (Evidence, error) {
	started := time.Now().UTC()
	evidence := Evidence{
		Name: cfg.Name, Environment: cfg.Environment, TargetURL: cfg.WebURL,
		RunID: cfg.RunID, GitSHA: cfg.GitSHA, StartedAt: started, Checks: make([]EvidenceCheck, 0, 6),
	}

	webProject, webProjectOutcome := inspectProject(ctx, client, cfg, cfg.WebProject, "apps/web")
	marketingProject, marketingProjectOutcome := inspectProject(ctx, client, cfg, cfg.MarketingProject, "apps/marketing")
	evidence.Checks = append(evidence.Checks,
		toEvidenceCheck("web-project-linked", webProjectOutcome),
		toEvidenceCheck("marketing-project-linked", marketingProjectOutcome),
	)

	webEnv := failed("web_project_unavailable")
	if webProjectOutcome.passed {
		webEnv = inspectEnvironment(ctx, client, cfg, cfg.WebProject,
			[]string{"ENVIRONMENT", "NEXT_PUBLIC_API_URL", "NEXT_PUBLIC_APP_URL"}, nil)
	}
	marketingEnv := failed("marketing_project_unavailable")
	if marketingProjectOutcome.passed {
		marketingEnv = inspectEnvironment(ctx, client, cfg, cfg.MarketingProject,
			[]string{"ENVIRONMENT", "NEXT_PUBLIC_API_URL", "NEXT_PUBLIC_APP_URL"},
			[]string{"AURAEDU_API_URL", "API_GATEWAY_URL"})
	}
	evidence.Checks = append(evidence.Checks, toEvidenceCheck("environment-configured", combine(webEnv, marketingEnv)))

	webDeployment := failed("web_project_unavailable")
	if webProjectOutcome.passed {
		webDeployment = inspectDeployment(ctx, client, cfg, webProject.ID)
	}
	webPublic := inspectPublicApp(ctx, client, cfg, cfg.WebURL, "/login", "Welcome back")
	evidence.Checks = append(evidence.Checks,
		toEvidenceCheck("web-production-deployed", combine(webDeployment, webPublic)))

	marketingDeployment := failed("marketing_project_unavailable")
	if marketingProjectOutcome.passed {
		marketingDeployment = inspectDeployment(ctx, client, cfg, marketingProject.ID)
	}
	marketingPublic := inspectPublicApp(ctx, client, cfg, cfg.MarketingURL, "/", "Run your school clearly")
	evidence.Checks = append(evidence.Checks,
		toEvidenceCheck("marketing-production-deployed", combine(marketingDeployment, marketingPublic)))

	corsOutcome := inspectGatewayCORS(ctx, client, cfg)
	evidence.Checks = append(evidence.Checks, toEvidenceCheck("gateway-cors-observed", corsOutcome))

	evidence.FinishedAt = time.Now().UTC()
	evidence.AllPassed = len(evidence.Checks) == 6
	failures := make([]string, 0)
	for _, check := range evidence.Checks {
		evidence.AllPassed = evidence.AllPassed && check.Passed
	}
	if !evidence.AllPassed {
		for _, outcome := range []checkOutcome{webProjectOutcome, marketingProjectOutcome, webEnv, marketingEnv,
			webDeployment, webPublic, marketingDeployment, marketingPublic, corsOutcome} {
			if !outcome.passed && outcome.failure != "" {
				failures = append(failures, outcome.failure)
			}
		}
		return evidence, fmt.Errorf("vercel production proof failed: %s", strings.Join(unique(failures), ", "))
	}
	return evidence, nil
}

func inspectProject(ctx context.Context, client *http.Client, cfg Config, project, expectedRoot string) (projectRecord, checkOutcome) {
	var record projectRecord
	if err := getVercelJSON(ctx, client, cfg, "/v9/projects/"+url.PathEscape(project), nil, &record); err != nil {
		return record, failed("project_lookup_failed")
	}
	linked := len(record.Link) > 0 && string(record.Link) != "null" && string(record.Link) != "{}"
	if record.ID == "" || record.Name == "" || record.Framework != "nextjs" || record.RootDirectory != expectedRoot || !linked {
		return record, failed("project_configuration_invalid")
	}
	return record, passed(strings.Join([]string{record.ID, record.Name, record.Framework, record.RootDirectory, fingerprint(string(record.Link))}, "|"))
}

func inspectEnvironment(
	ctx context.Context,
	client *http.Client,
	cfg Config,
	project string,
	required []string,
	oneOf []string,
) checkOutcome {
	var raw json.RawMessage
	if err := getVercelJSON(ctx, client, cfg, "/v10/projects/"+url.PathEscape(project)+"/env", nil, &raw); err != nil {
		return failed("environment_lookup_failed")
	}
	entries, err := decodeEnvironment(raw)
	if err != nil {
		return failed("environment_response_invalid")
	}
	productionKeys := make(map[string]bool)
	for _, entry := range entries {
		if targetIncludesProduction(entry.Target) {
			productionKeys[entry.Key] = true
		}
	}
	for _, key := range required {
		if !productionKeys[key] {
			return failed("production_environment_incomplete")
		}
	}
	if len(oneOf) > 0 {
		found := false
		for _, key := range oneOf {
			found = found || productionKeys[key]
		}
		if !found {
			return failed("production_gateway_environment_missing")
		}
	}
	keys := make([]string, 0, len(productionKeys))
	for key := range productionKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return passed(project + "|" + strings.Join(keys, ","))
}

func decodeEnvironment(raw json.RawMessage) ([]environmentVariable, error) {
	var entries []environmentVariable
	if err := json.Unmarshal(raw, &entries); err == nil {
		return entries, nil
	}
	var wrapped struct {
		Envs []environmentVariable `json:"envs"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil || wrapped.Envs == nil {
		return nil, errors.New("unsupported Vercel environment response")
	}
	return wrapped.Envs, nil
}

func targetIncludesProduction(raw json.RawMessage) bool {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return single == "production"
	}
	var multiple []string
	if json.Unmarshal(raw, &multiple) == nil {
		for _, target := range multiple {
			if target == "production" {
				return true
			}
		}
	}
	return false
}

func inspectDeployment(ctx context.Context, client *http.Client, cfg Config, projectID string) checkOutcome {
	query := url.Values{
		"projectId": {projectID}, "target": {"production"}, "state": {"READY"},
		"sha": {cfg.GitSHA}, "limit": {"1"},
	}
	var response struct {
		Deployments []deploymentRecord `json:"deployments"`
	}
	if err := getVercelJSON(ctx, client, cfg, "/v7/deployments", query, &response); err != nil {
		return failed("deployment_lookup_failed")
	}
	if len(response.Deployments) != 1 {
		return failed("release_deployment_missing")
	}
	deployment := response.Deployments[0]
	state := deployment.ReadyState
	if state == "" {
		state = deployment.State
	}
	if deployment.UID == "" || deployment.ProjectID != projectID || deployment.URL == "" || state != "READY" ||
		(deployment.Target != "" && deployment.Target != "production") {
		return failed("release_deployment_invalid")
	}
	if sha := deployment.Meta["githubCommitSha"]; sha != "" && sha != cfg.GitSHA {
		return failed("release_sha_mismatch")
	}
	return passed(strings.Join([]string{deployment.UID, deployment.ProjectID, deployment.URL, state, cfg.GitSHA}, "|"))
}

func inspectPublicApp(ctx context.Context, client *http.Client, cfg Config, origin, path, marker string) checkOutcome {
	parsed, err := url.Parse(origin)
	if err != nil {
		return failed("public_origin_invalid")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(origin, "/")+path, nil)
	if err != nil {
		return failed("public_request_invalid")
	}
	publicClient := *client
	redirects := 0
	publicClient.CheckRedirect = func(next *http.Request, _ []*http.Request) error {
		redirects++
		if redirects > 5 || !strings.EqualFold(next.URL.Scheme, parsed.Scheme) || !strings.EqualFold(next.URL.Host, parsed.Host) {
			return errors.New("redirect left expected application origin")
		}
		return nil
	}
	response, err := publicClient.Do(request)
	if err != nil {
		return failed("public_application_unreachable")
	}
	body, readErr := readBounded(response, cfg.MaxResponseBytes)
	if readErr != nil {
		return failed("public_response_invalid")
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), marker) || !securityHeadersPresent(response.Header) {
		return failed("public_application_contract_invalid")
	}
	return passed(strings.Join([]string{response.Request.URL.String(), marker, securityHeaderMaterial(response.Header)}, "|"))
}

func securityHeadersPresent(header http.Header) bool {
	permissions := strings.ToLower(header.Get("Permissions-Policy"))
	return strings.Contains(strings.ToLower(header.Get("Strict-Transport-Security")), "max-age=63072000") &&
		strings.EqualFold(header.Get("X-Content-Type-Options"), "nosniff") &&
		strings.EqualFold(header.Get("X-Frame-Options"), "DENY") &&
		strings.EqualFold(header.Get("Referrer-Policy"), "strict-origin-when-cross-origin") &&
		strings.Contains(permissions, "camera=()") && strings.Contains(permissions, "microphone=()") &&
		strings.Contains(permissions, "geolocation=()")
}

func securityHeaderMaterial(header http.Header) string {
	keys := []string{"Strict-Transport-Security", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy", "Permissions-Policy"}
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, strings.ToLower(header.Get(key)))
	}
	return strings.Join(values, "|")
}

func inspectGatewayCORS(ctx context.Context, client *http.Client, cfg Config) checkOutcome {
	origins := []string{strings.TrimRight(cfg.WebURL, "/"), strings.TrimRight(cfg.MarketingURL, "/")}
	observations := make([]string, 0, len(origins))
	for _, origin := range origins {
		request, err := http.NewRequestWithContext(ctx, http.MethodOptions,
			strings.TrimRight(cfg.GatewayURL, "/")+"/api/v1/public/billing/plans", nil)
		if err != nil {
			return failed("gateway_preflight_invalid")
		}
		request.Header.Set("Origin", origin)
		request.Header.Set("Access-Control-Request-Method", http.MethodGet)
		request.Header.Set("Access-Control-Request-Headers", "Content-Type,X-Request-Id")
		response, err := client.Do(request)
		if err != nil {
			return failed("gateway_preflight_unreachable")
		}
		_, readErr := readBounded(response, cfg.MaxResponseBytes)
		if readErr != nil || response.StatusCode != http.StatusNoContent ||
			response.Header.Get("Access-Control-Allow-Origin") != origin ||
			!listContains(response.Header.Get("Access-Control-Allow-Methods"), http.MethodGet) ||
			!listContains(response.Header.Get("Vary"), "Origin") {
			return failed("gateway_cors_contract_invalid")
		}
		observations = append(observations, origin+"|"+response.Header.Get("Access-Control-Allow-Methods"))
	}
	return passed(strings.Join(observations, "|"))
}

func listContains(raw, expected string) bool {
	for _, value := range strings.Split(raw, ",") {
		if strings.EqualFold(strings.TrimSpace(value), expected) {
			return true
		}
	}
	return false
}

func getVercelJSON(ctx context.Context, client *http.Client, cfg Config, path string, query url.Values, target any) error {
	endpoint, err := url.Parse(strings.TrimRight(cfg.VercelAPIBase, "/") + path)
	if err != nil {
		return err
	}
	if query == nil {
		query = make(url.Values)
	}
	if cfg.TeamID != "" {
		query.Set("teamId", cfg.TeamID)
	}
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+cfg.Token)
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	body, err := readBounded(response, cfg.MaxResponseBytes)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("vercel API returned HTTP %d", response.StatusCode)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return errors.New("vercel API returned invalid JSON")
	}
	return nil
}

func readBounded(response *http.Response, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	closeErr := response.Body.Close()
	if err != nil || closeErr != nil {
		return nil, errors.Join(err, closeErr)
	}
	if int64(len(body)) > limit {
		return nil, errors.New("response exceeded the configured size limit")
	}
	return body, nil
}

func passed(material string) checkOutcome {
	return checkOutcome{passed: true, material: material}
}

func failed(reason string) checkOutcome {
	return checkOutcome{failure: reason, material: reason}
}

func combine(outcomes ...checkOutcome) checkOutcome {
	materials := make([]string, 0, len(outcomes))
	failures := make([]string, 0)
	for _, outcome := range outcomes {
		materials = append(materials, outcome.material)
		if !outcome.passed {
			failures = append(failures, outcome.failure)
		}
	}
	if len(failures) > 0 {
		return checkOutcome{material: strings.Join(materials, "|"), failure: strings.Join(unique(failures), "+")}
	}
	return passed(strings.Join(materials, "|"))
}

func toEvidenceCheck(name string, outcome checkOutcome) EvidenceCheck {
	return EvidenceCheck{
		Name: name, Passed: outcome.passed, ObservedAt: time.Now().UTC(),
		EvidenceFingerprint: fingerprint(name + "|" + outcome.material),
	}
}

func unique(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func fingerprint(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])[:16]
}

func writeEvidence(path string, data []byte) error {
	// #nosec G304 -- the operator explicitly selects this exclusive local evidence path.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return errors.Join(err, file.Close())
	}
	return file.Close()
}
