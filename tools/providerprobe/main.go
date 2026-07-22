// Command providerprobe proves that a deployed AuraEDU notification service handed an email to its configured provider.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type Config struct {
	Name            string   `json:"name"`
	Environment     string   `json:"environment"`
	BaseURL         string   `json:"base_url"`
	Timeout         duration `json:"timeout"`
	DeliveryTimeout duration `json:"delivery_timeout"`
	PollInterval    duration `json:"poll_interval"`
	RunID           string   `json:"-"`
	GitSHA          string   `json:"-"`
	Tenant          string   `json:"-"`
	Token           string   `json:"-"`
	RecipientID     string   `json:"-"`
	Email           string   `json:"-"`
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

type messageRecord struct {
	ID               string     `json:"id"`
	Status           string     `json:"status"`
	SentAt           *time.Time `json:"sent_at"`
	Channel          string     `json:"channel"`
	TenantID         string     `json:"tenant_id"`
	Provider         *string    `json:"provider"`
	DeliveryStatus   *string    `json:"delivery_status"`
	DeliveryStatusAt *time.Time `json:"delivery_status_at"`
}

type StepResult struct {
	Name       string `json:"name"`
	StatusCode int    `json:"status_code"`
	DurationMS int64  `json:"duration_ms"`
	Passed     bool   `json:"passed"`
	Failure    string `json:"failure,omitempty"`
}

type Evidence struct {
	Name                 string       `json:"name"`
	Environment          string       `json:"environment"`
	BaseURL              string       `json:"base_url"`
	RunID                string       `json:"run_id"`
	GitSHA               string       `json:"git_sha"`
	StartedAt            time.Time    `json:"started_at"`
	FinishedAt           time.Time    `json:"finished_at"`
	TenantFingerprint    string       `json:"tenant_fingerprint"`
	RecipientFingerprint string       `json:"recipient_fingerprint"`
	MessageFingerprint   string       `json:"message_fingerprint"`
	Channel              string       `json:"channel"`
	Provider             string       `json:"provider"`
	ProviderOutcome      string       `json:"provider_outcome"`
	PersistedStatus      string       `json:"persisted_status"`
	SentAt               *time.Time   `json:"sent_at"`
	DeliveryStatus       string       `json:"delivery_status"`
	DeliveredAt          *time.Time   `json:"delivered_at"`
	AllPassed            bool         `json:"all_passed"`
	Steps                []StepResult `json:"steps"`
}

var (
	runIDPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{7,127}$`)
	gitSHAPattern = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
	uuidPattern   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
)

func main() {
	configPath := flag.String("config", "", "provider scenario JSON file")
	resultPath := flag.String("out", "", "immutable JSON evidence path")
	validateOnly := flag.Bool("validate-only", false, "validate versioned scenario without sending email")
	flag.Parse()
	if *configPath == "" {
		fatal(2, "-config is required")
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fatal(2, err.Error())
	}
	if *validateOnly {
		fmt.Printf("provider scenario %q valid for %s\n", cfg.Name, cfg.Environment)
		return
	}
	if *resultPath == "" {
		fatal(2, "-out is required for a staging provider run")
	}
	if err := validateExecution(cfg); err != nil {
		fatal(2, err.Error())
	}
	evidence, runErr := run(context.Background(), cfg)
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
	// #nosec G304 -- the operator explicitly selects this local provider scenario file.
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read scenario: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse scenario: %w", err)
	}
	if value := strings.TrimSpace(os.Getenv("AURA_PROVIDER_BASE_URL")); value != "" {
		cfg.BaseURL = value
	}
	cfg.RunID = strings.TrimSpace(os.Getenv("AURA_PROVIDER_RUN_ID"))
	cfg.GitSHA = strings.TrimSpace(os.Getenv("AURA_PROVIDER_GIT_SHA"))
	cfg.Tenant = strings.TrimSpace(os.Getenv("AURA_PROVIDER_TENANT"))
	cfg.Token = strings.TrimSpace(os.Getenv("AURA_PROVIDER_TOKEN"))
	cfg.RecipientID = strings.TrimSpace(os.Getenv("AURA_PROVIDER_RECIPIENT_ID"))
	cfg.Email = strings.TrimSpace(os.Getenv("AURA_PROVIDER_EMAIL"))
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validate(cfg Config) error {
	if cfg.Name != "auraedu-staging-email-provider" || cfg.Environment != "staging" || strings.TrimSpace(cfg.BaseURL) == "" {
		return errors.New("canonical name, staging environment and base_url are required")
	}
	if cfg.Timeout.Duration < time.Second || cfg.Timeout.Duration > 30*time.Second {
		return errors.New("timeout must be between one and thirty seconds")
	}
	if cfg.DeliveryTimeout.Duration < 10*time.Second || cfg.DeliveryTimeout.Duration > 10*time.Minute {
		return errors.New("delivery_timeout must be between ten seconds and ten minutes")
	}
	if cfg.PollInterval.Duration < 250*time.Millisecond || cfg.PollInterval.Duration > 10*time.Second {
		return errors.New("poll_interval must be between 250 milliseconds and ten seconds")
	}
	return nil
}

func validateExecution(cfg Config) error {
	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil || !isHTTPSOrigin(parsed) {
		return errors.New("provider proof requires a credential-free HTTPS origin")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasSuffix(host, ".example") {
		return errors.New("provider proof cannot target a placeholder or loopback host")
	}
	if !runIDPattern.MatchString(cfg.RunID) || !gitSHAPattern.MatchString(cfg.GitSHA) {
		return errors.New("AURA_PROVIDER_RUN_ID and AURA_PROVIDER_GIT_SHA are required")
	}
	if !hasRuntimeIdentity(cfg) {
		return errors.New("runtime tenant, bearer token and recipient UUID are required")
	}
	if !validMailbox(cfg.Email) {
		return errors.New("AURA_PROVIDER_EMAIL must be a plain valid mailbox")
	}
	return nil
}

func isHTTPSOrigin(parsed *url.URL) bool {
	return parsed.Scheme == "https" && parsed.Hostname() != "" && parsed.User == nil &&
		parsed.RawQuery == "" && parsed.Fragment == "" && (parsed.Path == "" || parsed.Path == "/")
}

func hasRuntimeIdentity(cfg Config) bool {
	return cfg.Tenant != "" && len(cfg.Token) >= 20 &&
		!strings.Contains(strings.ToLower(cfg.Token), "replace") && uuidPattern.MatchString(cfg.RecipientID)
}

func validMailbox(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && strings.EqualFold(address.Address, value) && !strings.ContainsAny(value, "\r\n")
}

func run(ctx context.Context, cfg Config) (Evidence, error) {
	started := time.Now().UTC()
	evidence := Evidence{
		Name: cfg.Name, Environment: cfg.Environment, BaseURL: strings.TrimRight(cfg.BaseURL, "/"), RunID: cfg.RunID, GitSHA: cfg.GitSHA,
		StartedAt: started, TenantFingerprint: fingerprint(cfg.Tenant), RecipientFingerprint: fingerprint(strings.ToLower(cfg.Email)),
		Channel: "email", ProviderOutcome: "not_accepted", PersistedStatus: "unknown",
		DeliveryStatus: "unknown", Steps: make([]StepResult, 0, 4),
	}
	client := &http.Client{Timeout: cfg.Timeout.Duration}
	messageID, createStep := createMessage(ctx, client, cfg)
	evidence.Steps = append(evidence.Steps, createStep)
	if !createStep.Passed {
		return finishEvidence(evidence, errors.New("provider proof failed during message creation"))
	}
	evidence.MessageFingerprint = fingerprint(messageID)
	sent, sendStep := sendMessage(ctx, client, cfg, messageID)
	evidence.Steps = append(evidence.Steps, sendStep)
	if !sendStep.Passed {
		return finishEvidence(evidence, errors.New("provider proof failed during provider handoff"))
	}
	fetched, fetchStep := getMessage(ctx, client, cfg, messageID)
	evidence.Steps = append(evidence.Steps, fetchStep)
	if !fetchStep.Passed {
		return finishEvidence(evidence, errors.New("provider proof failed while confirming persisted outcome"))
	}
	if !validAcceptedRecord(sent) || !validAcceptedRecord(fetched) {
		evidence.Steps[2].Passed = false
		evidence.Steps[2].Failure = "persisted_delivery_state_invalid"
		return finishEvidence(evidence, errors.New("provider acceptance was not persisted as sent"))
	}
	evidence.ProviderOutcome = "accepted"
	evidence.Provider = "resend"
	evidence.PersistedStatus = "sent"
	evidence.SentAt = fetched.SentAt
	delivered, deliveryStep := waitForDelivery(ctx, client, cfg, messageID)
	evidence.Steps = append(evidence.Steps, deliveryStep)
	if !deliveryStep.Passed {
		if delivered.DeliveryStatus != nil {
			evidence.DeliveryStatus = *delivered.DeliveryStatus
		}
		return finishEvidence(evidence, errors.New("provider delivery feedback was not confirmed"))
	}
	evidence.DeliveryStatus = "delivered"
	evidence.DeliveredAt = delivered.DeliveryStatusAt
	return finishEvidence(evidence, nil)
}

func validAcceptedRecord(record messageRecord) bool {
	if record.SentAt == nil || record.Status != "sent" || record.Channel != "email" ||
		record.Provider == nil || *record.Provider != "resend" || record.DeliveryStatus == nil ||
		record.DeliveryStatusAt == nil {
		return false
	}
	switch *record.DeliveryStatus {
	case "accepted", "delayed", "delivered":
		return true
	default:
		return false
	}
}

func finishEvidence(evidence Evidence, runErr error) (Evidence, error) {
	evidence.FinishedAt = time.Now().UTC()
	evidence.AllPassed = runErr == nil && len(evidence.Steps) == 4
	for _, step := range evidence.Steps {
		evidence.AllPassed = evidence.AllPassed && step.Passed
	}
	return evidence, runErr
}

func createMessage(ctx context.Context, client *http.Client, cfg Config) (string, StepResult) {
	payload := map[string]any{
		"recipient_id": cfg.RecipientID, "channel": "email",
		"subject":  "AuraEDU release delivery verification " + cfg.RunID,
		"body":     "This email verifies the AuraEDU staging notification-provider handoff. No action is required.",
		"metadata": map[string]any{"delivery_address": cfg.Email, "release_probe": true, "run_id": cfg.RunID},
	}
	var record messageRecord
	step := doJSON(ctx, client, cfg, http.MethodPost, "/api/v1/messages", payload, http.StatusCreated, &record, "create-message")
	if step.Passed && (!uuidPattern.MatchString(record.ID) || record.Status != "pending" || record.Channel != "email") {
		step.Passed = false
		step.Failure = "created_message_state_invalid"
	}
	return record.ID, step
}

func sendMessage(ctx context.Context, client *http.Client, cfg Config, messageID string) (messageRecord, StepResult) {
	var record messageRecord
	step := doJSON(ctx, client, cfg, http.MethodPost, "/api/v1/messages/"+url.PathEscape(messageID)+"/send", nil, http.StatusOK, &record, "provider-handoff")
	return record, step
}

func getMessage(ctx context.Context, client *http.Client, cfg Config, messageID string) (messageRecord, StepResult) {
	var record messageRecord
	step := doJSON(ctx, client, cfg, http.MethodGet, "/api/v1/messages/"+url.PathEscape(messageID), nil, http.StatusOK, &record, "persisted-outcome")
	return record, step
}

func waitForDelivery(ctx context.Context, client *http.Client, cfg Config, messageID string) (messageRecord, StepResult) {
	started := time.Now()
	deadline := time.NewTimer(cfg.DeliveryTimeout.Duration)
	defer deadline.Stop()
	var latest messageRecord
	result := StepResult{Name: "delivery-feedback"}
	for {
		record, attempt := getMessage(ctx, client, cfg, messageID)
		latest = record
		result.StatusCode = attempt.StatusCode
		if attempt.Passed && record.Provider != nil && *record.Provider == "resend" &&
			record.DeliveryStatus != nil && *record.DeliveryStatus == "delivered" &&
			record.DeliveryStatusAt != nil {
			result.Passed = true
			result.DurationMS = time.Since(started).Milliseconds()
			return latest, result
		}
		if attempt.Passed && record.DeliveryStatus != nil {
			switch *record.DeliveryStatus {
			case "bounced", "complained", "failed", "suppressed":
				result.Failure = "terminal_delivery_failure"
				result.DurationMS = time.Since(started).Milliseconds()
				return latest, result
			}
		}
		wait := time.NewTimer(cfg.PollInterval.Duration)
		select {
		case <-ctx.Done():
			stopTimer(wait)
			result.Failure = "context_cancelled"
			result.DurationMS = time.Since(started).Milliseconds()
			return latest, result
		case <-deadline.C:
			stopTimer(wait)
			result.Failure = "delivery_feedback_timeout"
			result.DurationMS = time.Since(started).Milliseconds()
			return latest, result
		case <-wait.C:
		}
	}
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func doJSON(ctx context.Context, client *http.Client, cfg Config, method, path string, payload any, expected int, target any, name string) StepResult {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return StepResult{Name: name, Failure: "request_encoding_failed"}
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(cfg.BaseURL, "/")+path, body)
	if err != nil {
		return StepResult{Name: name, Failure: "request_creation_failed"}
	}
	request.Header.Set("Authorization", "Bearer "+cfg.Token)
	request.Header.Set("X-Tenant-Code", cfg.Tenant)
	request.Header.Set("Content-Type", "application/json")
	started := time.Now()
	response, err := client.Do(request)
	result := StepResult{Name: name, DurationMS: time.Since(started).Milliseconds()}
	if err != nil {
		result.Failure = "transport_failure"
		return result
	}
	result.StatusCode = response.StatusCode
	limited, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	closeErr := response.Body.Close()
	if err != nil || closeErr != nil {
		result.Failure = "response_read_failed"
		return result
	}
	if response.StatusCode != expected {
		result.Failure = "unexpected_status"
		return result
	}
	if err := json.Unmarshal(limited, target); err != nil {
		result.Failure = "invalid_response"
		return result
	}
	result.Passed = true
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
