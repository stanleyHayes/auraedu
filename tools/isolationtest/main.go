// Command isolationtest proves deployed two-school HTTP isolation without retaining credentials or resource IDs.
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
	"slices"
	"strings"
	"time"
)

type Config struct {
	Name              string   `json:"name"`
	Environment       string   `json:"environment"`
	BaseURL           string   `json:"base_url"`
	Timeout           duration `json:"timeout"`
	MinimumProbeCount int      `json:"minimum_probe_count"`
	Schools           []School `json:"schools"`
	Probes            []Probe  `json:"probes"`
	RunID             string   `json:"-"`
	GitSHA            string   `json:"-"`
}

type School struct {
	Code      string            `json:"code"`
	Token     string            `json:"token"`
	Resources map[string]string `json:"resources"`
}

type Probe struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	ResourceKey    string `json:"resource_key"`
	OwnStatus      []int  `json:"own_status"`
	CrossStatus    []int  `json:"cross_status"`
	MismatchStatus []int  `json:"mismatch_status"`
}

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

type CheckResult struct {
	Probe      string `json:"probe"`
	Direction  string `json:"direction"`
	Kind       string `json:"kind"`
	StatusCode int    `json:"status_code"`
	DurationMS int64  `json:"duration_ms"`
	Passed     bool   `json:"passed"`
	Failure    string `json:"failure,omitempty"`
}

type Evidence struct {
	Name           string        `json:"name"`
	Environment    string        `json:"environment"`
	BaseURL        string        `json:"base_url"`
	RunID          string        `json:"run_id"`
	GitSHA         string        `json:"git_sha"`
	StartedAt      time.Time     `json:"started_at"`
	FinishedAt     time.Time     `json:"finished_at"`
	SchoolCount    int           `json:"school_count"`
	SchoolHashes   []string      `json:"school_fingerprints"`
	ProbeCount     int           `json:"probe_count"`
	ExpectedChecks int           `json:"expected_checks"`
	PassedChecks   int           `json:"passed_checks"`
	FailedChecks   int           `json:"failed_checks"`
	AllPassed      bool          `json:"all_passed"`
	Checks         []CheckResult `json:"checks"`
}

var (
	tenantCodePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)
	runIDPattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{7,127}$`)
	gitSHAPattern     = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
	placeholderToken  = regexp.MustCompile(`(?i)(replace|placeholder|change[-_]?me|example|token[-_][ab])`)
)

func main() {
	configPath := flag.String("config", "", "isolation scenario JSON file")
	resultPath := flag.String("out", "", "immutable JSON evidence path")
	validateOnly := flag.Bool("validate-only", false, "validate scenario shape without sending traffic")
	flag.Parse()
	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "-config is required")
		os.Exit(2)
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *validateOnly {
		fmt.Printf("isolation scenario %q valid: %d schools, %d probes, %d checks\n", cfg.Name, len(cfg.Schools), len(cfg.Probes), len(cfg.Probes)*6)
		return
	}
	if *resultPath == "" {
		fmt.Fprintln(os.Stderr, "-out is required for a staging isolation run")
		os.Exit(2)
	}
	if err := validateExecution(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	evidence, runErr := run(context.Background(), cfg)
	encoded, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode evidence:", err)
		os.Exit(1)
	}
	if err := writeEvidence(*resultPath, append(encoded, '\n')); err != nil {
		fmt.Fprintln(os.Stderr, "write evidence:", err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
		os.Exit(1)
	}
}

func loadConfig(path string) (Config, error) {
	// #nosec G304 -- the operator explicitly selects this local scenario file.
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read scenario: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse scenario: %w", err)
	}
	if value := strings.TrimSpace(os.Getenv("AURA_ISOLATION_BASE_URL")); value != "" {
		cfg.BaseURL = value
	}
	if value := strings.TrimSpace(os.Getenv("AURA_ISOLATION_SCHOOLS_JSON")); value != "" {
		if err := json.Unmarshal([]byte(value), &cfg.Schools); err != nil {
			return Config{}, fmt.Errorf("parse AURA_ISOLATION_SCHOOLS_JSON: %w", err)
		}
	}
	cfg.RunID = strings.TrimSpace(os.Getenv("AURA_ISOLATION_RUN_ID"))
	cfg.GitSHA = strings.TrimSpace(os.Getenv("AURA_ISOLATION_GIT_SHA"))
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validate(cfg Config) error {
	if cfg.Name == "" || cfg.Environment != "staging" || strings.TrimSpace(cfg.BaseURL) == "" {
		return errors.New("name, staging environment and base_url are required")
	}
	if cfg.Timeout.Duration <= 0 || cfg.MinimumProbeCount < 8 || len(cfg.Probes) < cfg.MinimumProbeCount {
		return errors.New("positive timeout and at least the configured eight probes are required")
	}
	if len(cfg.Schools) != 2 {
		return fmt.Errorf("exactly two schools are required, got %d", len(cfg.Schools))
	}
	if err := validateSchools(cfg.Schools); err != nil {
		return err
	}
	return validateProbes(cfg)
}

func validateSchools(schools []School) error {
	seenSchools := map[string]struct{}{}
	for _, school := range schools {
		if !tenantCodePattern.MatchString(school.Code) {
			return fmt.Errorf("school code %q is not canonical", school.Code)
		}
		if _, exists := seenSchools[school.Code]; exists {
			return fmt.Errorf("duplicate school code %q", school.Code)
		}
		seenSchools[school.Code] = struct{}{}
	}
	return nil
}

func validateProbes(cfg Config) error {
	seenProbes := map[string]struct{}{}
	seenResources := map[string]struct{}{}
	for _, probe := range cfg.Probes {
		if probe.Name == "" || probe.ResourceKey == "" || !strings.HasPrefix(probe.Path, "/api/v1/") || strings.Count(probe.Path, "{resource}") != 1 {
			return fmt.Errorf("invalid probe %q", probe.Name)
		}
		if _, exists := seenProbes[probe.Name]; exists {
			return fmt.Errorf("duplicate probe %q", probe.Name)
		}
		seenProbes[probe.Name] = struct{}{}
		if _, exists := seenResources[probe.ResourceKey]; exists {
			return fmt.Errorf("resource key %q is reused", probe.ResourceKey)
		}
		seenResources[probe.ResourceKey] = struct{}{}
		validControls := slices.Equal(probe.OwnStatus, []int{http.StatusOK}) &&
			slices.Equal(probe.CrossStatus, []int{http.StatusNotFound}) &&
			slices.Equal(probe.MismatchStatus, []int{http.StatusForbidden})
		if !validControls {
			return fmt.Errorf("probe %q must require exact 200/404/403 controls", probe.Name)
		}
		for _, school := range cfg.Schools {
			if strings.TrimSpace(school.Resources[probe.ResourceKey]) == "" {
				return fmt.Errorf("school %q is missing resource %q", school.Code, probe.ResourceKey)
			}
		}
		if cfg.Schools[0].Resources[probe.ResourceKey] == cfg.Schools[1].Resources[probe.ResourceKey] {
			return fmt.Errorf("resource %q must differ between schools", probe.ResourceKey)
		}
	}
	return nil
}

func validateExecution(cfg Config) error {
	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil || !isHTTPSOrigin(parsed) {
		return errors.New("staging isolation requires a credential-free HTTPS origin")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasSuffix(host, ".example") {
		return errors.New("staging isolation cannot target a placeholder or loopback host")
	}
	if !runIDPattern.MatchString(cfg.RunID) || !gitSHAPattern.MatchString(cfg.GitSHA) {
		return errors.New("AURA_ISOLATION_RUN_ID and AURA_ISOLATION_GIT_SHA are required")
	}
	for _, school := range cfg.Schools {
		if len(school.Token) < 20 || placeholderToken.MatchString(school.Token) {
			return fmt.Errorf("school %q requires a runtime bearer token", school.Code)
		}
	}
	return nil
}

func isHTTPSOrigin(parsed *url.URL) bool {
	return parsed.Scheme == "https" && parsed.Hostname() != "" && parsed.User == nil &&
		parsed.RawQuery == "" && parsed.Fragment == "" && (parsed.Path == "" || parsed.Path == "/")
}

func run(ctx context.Context, cfg Config) (Evidence, error) {
	started := time.Now().UTC()
	evidence := Evidence{
		Name: cfg.Name, Environment: cfg.Environment, BaseURL: strings.TrimRight(cfg.BaseURL, "/"),
		RunID: cfg.RunID, GitSHA: cfg.GitSHA, StartedAt: started, SchoolCount: 2,
		SchoolHashes: []string{fingerprint(cfg.Schools[0].Code), fingerprint(cfg.Schools[1].Code)},
		ProbeCount:   len(cfg.Probes), ExpectedChecks: len(cfg.Probes) * 6,
		Checks: make([]CheckResult, 0, len(cfg.Probes)*6),
	}
	client := &http.Client{Timeout: cfg.Timeout.Duration}
	for actorIndex := range cfg.Schools {
		ownerIndex := 1 - actorIndex
		actor := cfg.Schools[actorIndex]
		owner := cfg.Schools[ownerIndex]
		direction := fmt.Sprintf("school-%d-to-school-%d", actorIndex+1, ownerIndex+1)
		for _, probe := range cfg.Probes {
			evidence.Checks = append(evidence.Checks,
				execute(ctx, client, cfg.BaseURL, actor, actor, probe, "own-control", direction, probe.OwnStatus),
				execute(ctx, client, cfg.BaseURL, actor, owner, probe, "cross-resource", direction, probe.CrossStatus),
				executeMismatch(ctx, client, cfg.BaseURL, actor, owner, probe, direction),
			)
		}
	}
	for _, check := range evidence.Checks {
		if check.Passed {
			evidence.PassedChecks++
		} else {
			evidence.FailedChecks++
		}
	}
	evidence.AllPassed = evidence.FailedChecks == 0 && len(evidence.Checks) == evidence.ExpectedChecks
	evidence.FinishedAt = time.Now().UTC()
	if !evidence.AllPassed {
		return evidence, fmt.Errorf("isolation matrix failed %d of %d checks", evidence.FailedChecks, evidence.ExpectedChecks)
	}
	return evidence, nil
}

func execute(
	ctx context.Context,
	client *http.Client,
	base string,
	actor School,
	resourceOwner School,
	probe Probe,
	kind string,
	direction string,
	expected []int,
) CheckResult {
	resource := resourceOwner.Resources[probe.ResourceKey]
	path := strings.ReplaceAll(probe.Path, "{resource}", url.PathEscape(resource))
	return request(ctx, client, base+path, actor.Token, actor.Code, probe, kind, direction, expected, resourceOwner)
}

func executeMismatch(ctx context.Context, client *http.Client, base string, actor, other School, probe Probe, direction string) CheckResult {
	resource := actor.Resources[probe.ResourceKey]
	path := strings.ReplaceAll(probe.Path, "{resource}", url.PathEscape(resource))
	return request(ctx, client, base+path, actor.Token, other.Code, probe, "tenant-header-mismatch", direction, probe.MismatchStatus, other)
}

func request(
	ctx context.Context,
	client *http.Client,
	target string,
	token string,
	tenant string,
	probe Probe,
	kind string,
	direction string,
	expected []int,
	forbidden School,
) CheckResult {
	started := time.Now()
	result := CheckResult{Probe: probe.Name, Kind: kind, Direction: direction}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		result.Failure = "request_creation_failed"
		return result
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-Code", tenant)
	req.Header.Set("X-Tenant-ID", tenant)
	resp, err := client.Do(req)
	result.DurationMS = time.Since(started).Milliseconds()
	if err != nil {
		result.Failure = "transport_failure"
		return result
	}
	result.StatusCode = resp.StatusCode
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil {
		result.Failure = "response_read_failed"
		return result
	}
	if !slices.Contains(expected, resp.StatusCode) {
		result.Failure = "unexpected_status"
		return result
	}
	if kind != "own-control" && responseLeaks(body, forbidden) {
		result.Failure = "denial_response_disclosed_tenant_or_resource"
		return result
	}
	result.Passed = true
	return result
}

func responseLeaks(body []byte, school School) bool {
	text := strings.ToLower(string(body))
	if strings.Contains(text, strings.ToLower(school.Code)) {
		return true
	}
	for _, resource := range school.Resources {
		if len(resource) >= 8 && strings.Contains(text, strings.ToLower(resource)) {
			return true
		}
	}
	return false
}

func fingerprint(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])[:16]
}

func writeEvidence(path string, data []byte) error {
	// #nosec G304 -- the operator explicitly selects this local, exclusive evidence path.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return errors.Join(err, file.Close())
	}
	return file.Close()
}
