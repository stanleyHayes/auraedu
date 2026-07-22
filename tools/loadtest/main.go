// Command loadtest runs thresholded HTTP load scenarios against an AuraEDU environment.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	Name          string        `json:"name"`
	Environment   string        `json:"environment"`
	BaseURL       string        `json:"base_url"`
	Duration      duration      `json:"duration"`
	Concurrency   int           `json:"concurrency"`
	RatePerSecond float64       `json:"rate_per_second,omitempty"`
	MinThroughput float64       `json:"min_requests_per_second,omitempty"`
	Timeout       duration      `json:"timeout"`
	MaxErrorRate  float64       `json:"max_error_rate"`
	P95Millis     int64         `json:"p95_ms"`
	P99Millis     int64         `json:"p99_ms"`
	TenantCount   int           `json:"required_tenant_count"`
	RequireAuth   bool          `json:"require_authentication,omitempty"`
	Tenants       []Tenant      `json:"tenants"`
	Requests      []RequestSpec `json:"requests"`
	RunID         string        `json:"-"`
	GitSHA        string        `json:"-"`
}

type Tenant struct {
	Code  string `json:"code"`
	Token string `json:"token,omitempty"`
}

type RequestSpec struct {
	Name           string         `json:"name"`
	Method         string         `json:"method"`
	Path           string         `json:"path"`
	Body           map[string]any `json:"body,omitempty"`
	ExpectedStatus []int          `json:"expected_status"`
}

type duration struct{ time.Duration }

func (d *duration) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return err
	}
	d.Duration = value
	return nil
}

type sample struct {
	Name       string
	Tenant     string
	Duration   time.Duration
	StatusCode int
	Failed     bool
	Dropped    bool
}

type summary struct {
	Name                string             `json:"name"`
	Environment         string             `json:"environment"`
	BaseURL             string             `json:"base_url"`
	RunID               string             `json:"run_id,omitempty"`
	GitSHA              string             `json:"git_sha,omitempty"`
	StartedAt           time.Time          `json:"started_at"`
	FinishedAt          time.Time          `json:"finished_at"`
	ElapsedMS           int64              `json:"elapsed_ms"`
	ConfiguredDuration  string             `json:"configured_duration"`
	Concurrency         int                `json:"concurrency"`
	ConfiguredRate      float64            `json:"configured_requests_per_second"`
	MinimumThroughput   float64            `json:"minimum_requests_per_second"`
	RequiredTenantCount int                `json:"required_tenant_count"`
	ObservedTenantCount int                `json:"observed_tenant_count"`
	Dropped             int                `json:"dropped_at_shutdown"`
	Thresholds          resultThresholds   `json:"thresholds"`
	Requests            int                `json:"requests"`
	Failures            int                `json:"failures"`
	ErrorRate           float64            `json:"error_rate"`
	P50Millis           int64              `json:"p50_ms"`
	P95Millis           int64              `json:"p95_ms"`
	P99Millis           int64              `json:"p99_ms"`
	Throughput          float64            `json:"requests_per_second"`
	ByRequest           map[string]metrics `json:"by_request"`
	ByTenant            map[string]metrics `json:"by_tenant"`
}

type resultThresholds struct {
	MaxErrorRate float64 `json:"max_error_rate"`
	P95Millis    int64   `json:"p95_ms"`
	P99Millis    int64   `json:"p99_ms"`
}

type metrics struct {
	Requests   int     `json:"requests"`
	Failures   int     `json:"failures"`
	ErrorRate  float64 `json:"error_rate"`
	P50Millis  int64   `json:"p50_ms"`
	P95Millis  int64   `json:"p95_ms"`
	P99Millis  int64   `json:"p99_ms"`
	Throughput float64 `json:"requests_per_second"`
}

func main() {
	configPath := flag.String("config", "", "scenario JSON file")
	resultPath := flag.String("out", "", "optional JSON result path")
	validateOnly := flag.Bool("validate-only", false, "validate configuration without sending traffic")
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
		fmt.Printf("scenario %q valid: %d tenants, %d requests\n", cfg.Name, len(cfg.Tenants), len(cfg.Requests))
		return
	}
	if err := validateExecution(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	result, err := run(context.Background(), cfg)
	encoded, marshalErr := json.MarshalIndent(result, "", "  ")
	if marshalErr != nil {
		fmt.Fprintln(os.Stderr, "encode result:", marshalErr)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
	if *resultPath != "" {
		if writeErr := writeResult(*resultPath, append(encoded, '\n')); writeErr != nil {
			fmt.Fprintln(os.Stderr, "write result:", writeErr)
			os.Exit(1)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeResult(path string, data []byte) error {
	// #nosec G304 -- the operator explicitly selects this exclusive local result path.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return errors.Join(err, file.Close())
	}
	return file.Close()
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
	if value := strings.TrimSpace(os.Getenv("AURA_PERF_BASE_URL")); value != "" {
		cfg.BaseURL = value
	}
	if value := strings.TrimSpace(os.Getenv("AURA_PERF_TENANTS_JSON")); value != "" {
		if err := json.Unmarshal([]byte(value), &cfg.Tenants); err != nil {
			return Config{}, fmt.Errorf("parse AURA_PERF_TENANTS_JSON: %w", err)
		}
	}
	if value := strings.TrimSpace(os.Getenv("AURA_PERF_DURATION")); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse AURA_PERF_DURATION: %w", err)
		}
		cfg.Duration.Duration = parsed
	}
	if value := strings.TrimSpace(os.Getenv("AURA_PERF_CONCURRENCY")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse AURA_PERF_CONCURRENCY: %w", err)
		}
		cfg.Concurrency = parsed
	}
	cfg.RunID = strings.TrimSpace(os.Getenv("AURA_PERF_RUN_ID"))
	cfg.GitSHA = strings.TrimSpace(os.Getenv("AURA_PERF_GIT_SHA"))
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validate(cfg Config) error {
	if cfg.Name == "" || strings.TrimRight(cfg.BaseURL, "/") == "" {
		return errors.New("scenario name and base_url are required")
	}
	if cfg.Duration.Duration <= 0 || cfg.Concurrency <= 0 || cfg.Timeout.Duration <= 0 {
		return errors.New("duration, concurrency and timeout must be positive")
	}
	if cfg.RatePerSecond < 0 {
		return errors.New("rate_per_second cannot be negative")
	}
	if cfg.RatePerSecond > 0 && (cfg.MinThroughput <= 0 || cfg.MinThroughput > cfg.RatePerSecond) {
		return errors.New("paced scenarios require min_requests_per_second between zero and rate_per_second")
	}
	if cfg.MaxErrorRate < 0 || cfg.MaxErrorRate > 1 || cfg.P95Millis <= 0 || cfg.P99Millis < cfg.P95Millis {
		return errors.New("invalid error-rate or latency thresholds")
	}
	if len(cfg.Tenants) == 0 || len(cfg.Requests) == 0 {
		return errors.New("at least one tenant and request are required")
	}
	if cfg.TenantCount > 0 && len(cfg.Tenants) != cfg.TenantCount {
		return fmt.Errorf("scenario requires exactly %d tenants, got %d", cfg.TenantCount, len(cfg.Tenants))
	}
	if err := validateTenants(cfg.Tenants); err != nil {
		return err
	}
	return validateRequests(cfg.Requests)
}

func validateTenants(tenants []Tenant) error {
	seen := map[string]struct{}{}
	for _, tenant := range tenants {
		if !tenantCodePattern.MatchString(tenant.Code) {
			return fmt.Errorf("tenant code %q is not canonical", tenant.Code)
		}
		if _, ok := seen[tenant.Code]; ok {
			return fmt.Errorf("duplicate tenant %q", tenant.Code)
		}
		seen[tenant.Code] = struct{}{}
	}
	return nil
}

func validateRequests(requests []RequestSpec) error {
	for _, request := range requests {
		if request.Name == "" || !strings.HasPrefix(request.Path, "/") || len(request.ExpectedStatus) == 0 {
			return fmt.Errorf("invalid request spec %q", request.Name)
		}
	}
	return nil
}

var (
	tenantCodePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)
	runIDPattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{7,127}$`)
	gitSHAPattern     = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
)

func validateExecution(cfg Config) error {
	if cfg.RequireAuth {
		for _, tenant := range cfg.Tenants {
			if tenant.Token == "" || strings.Contains(strings.ToLower(tenant.Token), "replace") {
				return fmt.Errorf("tenant %q requires a runtime authentication token", tenant.Code)
			}
		}
	}
	if cfg.Environment != "staging" {
		return nil
	}
	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("staging performance runs require a credential-free HTTPS base URL without query or fragment")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "127.0.0.1" || strings.HasSuffix(host, ".example") {
		return errors.New("staging performance runs cannot target a placeholder or loopback host")
	}
	if !runIDPattern.MatchString(cfg.RunID) {
		return errors.New("staging performance runs require AURA_PERF_RUN_ID (8-128 safe characters)")
	}
	if !gitSHAPattern.MatchString(cfg.GitSHA) {
		return errors.New("staging performance runs require AURA_PERF_GIT_SHA")
	}
	return nil
}

func run(parent context.Context, cfg Config) (summary, error) {
	ctx, cancel := context.WithTimeout(parent, cfg.Duration.Duration)
	defer cancel()
	client := &http.Client{Timeout: cfg.Timeout.Duration}
	samples := make(chan sample, cfg.Concurrency*8)
	var pace <-chan time.Time
	var ticker *time.Ticker
	if cfg.RatePerSecond > 0 {
		interval := time.Duration(float64(time.Second) / cfg.RatePerSecond)
		if interval < time.Microsecond {
			interval = time.Microsecond
		}
		ticker = time.NewTicker(interval)
		pace = ticker.C
		defer ticker.Stop()
	}
	var sequence atomic.Uint64
	var wg sync.WaitGroup
	for worker := 0; worker < cfg.Concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				if pace != nil {
					select {
					case <-ctx.Done():
						return
					case <-pace:
					}
				}
				index := sequence.Add(1) - 1
				tenant := cfg.Tenants[index%uint64(len(cfg.Tenants))]
				spec := cfg.Requests[(index/uint64(len(cfg.Tenants)))%uint64(len(cfg.Requests))]
				samples <- execute(ctx, client, strings.TrimRight(cfg.BaseURL, "/"), tenant, spec, index)
			}
		}()
	}
	go func() { wg.Wait(); close(samples) }()
	started := time.Now()
	all := make([]sample, 0, cfg.Concurrency*100)
	dropped := 0
	for item := range samples {
		if item.Dropped {
			dropped++
			continue
		}
		all = append(all, item)
	}
	finished := time.Now()
	result := summarize(cfg, all, dropped, started, finished)
	if result.Requests == 0 {
		return result, errors.New("load scenario produced no requests")
	}
	var failures []string
	failures = append(failures, thresholdFailures("aggregate", metricsFromSummary(result), cfg)...)
	if result.Throughput < cfg.MinThroughput {
		failures = append(failures, fmt.Sprintf("aggregate throughput %.2f req/s is below %.2f req/s", result.Throughput, cfg.MinThroughput))
	}
	for name, value := range result.ByRequest {
		failures = append(failures, thresholdFailures("request "+name, value, cfg)...)
	}
	for tenant, value := range result.ByTenant {
		failures = append(failures, thresholdFailures("tenant "+tenant, value, cfg)...)
	}
	if len(failures) > 0 {
		return result, errors.New(strings.Join(failures, "; "))
	}
	return result, nil
}

func execute(ctx context.Context, client *http.Client, base string, tenant Tenant, spec RequestSpec, sequence uint64) sample {
	path := strings.ReplaceAll(spec.Path, "{tenant}", tenant.Code)
	path = strings.ReplaceAll(path, "{sequence}", strconv.FormatUint(sequence, 10))
	var body io.Reader
	if spec.Body != nil {
		encoded, err := json.Marshal(spec.Body)
		if err != nil {
			return sample{Name: spec.Name, Tenant: tenant.Code, Failed: true}
		}
		encoded = bytes.ReplaceAll(encoded, []byte("{tenant}"), []byte(tenant.Code))
		encoded = bytes.ReplaceAll(encoded, []byte("{sequence}"), []byte(strconv.FormatUint(sequence, 10)))
		body = bytes.NewReader(encoded)
	}
	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(spec.Method), base+path, body)
	if err != nil {
		return sample{Name: spec.Name, Tenant: tenant.Code, Duration: time.Since(started), Failed: true}
	}
	req.Header.Set("X-Tenant-Code", tenant.Code)
	req.Header.Set("X-Tenant-ID", tenant.Code)
	if tenant.Token != "" {
		req.Header.Set("Authorization", "Bearer "+tenant.Token)
	}
	if spec.Body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", fmt.Sprintf("perf-%s-%d", tenant.Code, sequence))
	}
	resp, err := client.Do(req)
	item := sample{Name: spec.Name, Tenant: tenant.Code, Duration: time.Since(started), Failed: err != nil}
	if err != nil {
		if ctx.Err() != nil {
			item.Dropped = true
		}
		return item
	}
	item.StatusCode = resp.StatusCode
	_, copyErr := io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	closeErr := resp.Body.Close()
	if copyErr != nil || closeErr != nil {
		item.Failed = true
		return item
	}
	item.Failed = !containsStatus(spec.ExpectedStatus, resp.StatusCode)
	return item
}

func containsStatus(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func summarize(cfg Config, samples []sample, dropped int, started, finished time.Time) summary {
	elapsed := finished.Sub(started)
	allMetrics := calculateMetrics(samples, elapsed)
	result := summary{
		Name: cfg.Name, Environment: cfg.Environment, BaseURL: strings.TrimRight(cfg.BaseURL, "/"),
		RunID: cfg.RunID, GitSHA: cfg.GitSHA, StartedAt: started.UTC(), FinishedAt: finished.UTC(),
		ElapsedMS: elapsed.Milliseconds(), ConfiguredDuration: cfg.Duration.String(),
		Concurrency: cfg.Concurrency, ConfiguredRate: cfg.RatePerSecond, MinimumThroughput: cfg.MinThroughput,
		RequiredTenantCount: cfg.TenantCount, Dropped: dropped,
		Thresholds: resultThresholds{MaxErrorRate: cfg.MaxErrorRate, P95Millis: cfg.P95Millis, P99Millis: cfg.P99Millis},
		Requests:   allMetrics.Requests, Failures: allMetrics.Failures,
		ErrorRate: allMetrics.ErrorRate, P50Millis: allMetrics.P50Millis,
		P95Millis: allMetrics.P95Millis, P99Millis: allMetrics.P99Millis,
		Throughput: allMetrics.Throughput, ByRequest: map[string]metrics{}, ByTenant: map[string]metrics{},
	}
	requests := map[string][]sample{}
	tenants := map[string][]sample{}
	for _, item := range samples {
		requests[item.Name] = append(requests[item.Name], item)
		tenants[item.Tenant] = append(tenants[item.Tenant], item)
	}
	for key, values := range requests {
		result.ByRequest[key] = calculateMetrics(values, elapsed)
	}
	for key, values := range tenants {
		result.ByTenant[key] = calculateMetrics(values, elapsed)
	}
	result.ObservedTenantCount = len(result.ByTenant)
	return result
}

func calculateMetrics(samples []sample, elapsed time.Duration) metrics {
	durations := make([]time.Duration, 0, len(samples))
	failures := 0
	for _, item := range samples {
		durations = append(durations, item.Duration)
		if item.Failed {
			failures++
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	result := metrics{Requests: len(samples), Failures: failures}
	if len(samples) > 0 {
		result.ErrorRate = float64(failures) / float64(len(samples))
		result.P50Millis = percentile(durations, 0.50).Milliseconds()
		result.P95Millis = percentile(durations, 0.95).Milliseconds()
		result.P99Millis = percentile(durations, 0.99).Milliseconds()
	}
	if elapsed > 0 {
		result.Throughput = float64(len(samples)) / elapsed.Seconds()
	}
	return result
}

func metricsFromSummary(value summary) metrics {
	return metrics{
		Requests: value.Requests, Failures: value.Failures, ErrorRate: value.ErrorRate,
		P50Millis: value.P50Millis, P95Millis: value.P95Millis, P99Millis: value.P99Millis,
		Throughput: value.Throughput,
	}
}

func thresholdFailures(scope string, value metrics, cfg Config) []string {
	var failures []string
	if value.ErrorRate > cfg.MaxErrorRate {
		failures = append(failures, fmt.Sprintf("%s error rate %.4f exceeds %.4f", scope, value.ErrorRate, cfg.MaxErrorRate))
	}
	if value.P95Millis > cfg.P95Millis {
		failures = append(failures, fmt.Sprintf("%s p95 %dms exceeds %dms", scope, value.P95Millis, cfg.P95Millis))
	}
	if value.P99Millis > cfg.P99Millis {
		failures = append(failures, fmt.Sprintf("%s p99 %dms exceeds %dms", scope, value.P99Millis, cfg.P99Millis))
	}
	return failures
}

func percentile(sorted []time.Duration, quantile float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	index := int(math.Ceil(quantile*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	return sorted[index]
}
