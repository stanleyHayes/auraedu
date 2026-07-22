// Command twilioprobe produces privacy-minimized release evidence for deployed
// AuraEDU SMS and WhatsApp delivery through Twilio.
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
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
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
	SMSNumber       string   `json:"-"`
	WhatsAppNumber  string   `json:"-"`
}

type messageRecord struct {
	ID               string     `json:"id"`
	Channel          string     `json:"channel"`
	Status           string     `json:"status"`
	Provider         *string    `json:"provider"`
	DeliveryStatus   *string    `json:"delivery_status"`
	SentAt           *time.Time `json:"sent_at"`
	DeliveryStatusAt *time.Time `json:"delivery_status_at"`
}

type Check struct {
	Name                string    `json:"name"`
	Passed              bool      `json:"passed"`
	ObservedAt          time.Time `json:"observed_at"`
	EvidenceFingerprint string    `json:"evidence_fingerprint"`
}

type Evidence struct {
	Name        string    `json:"name"`
	Environment string    `json:"environment"`
	TargetURL   string    `json:"target_url"`
	RunID       string    `json:"run_id"`
	GitSHA      string    `json:"git_sha"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at"`
	AllPassed   bool      `json:"all_passed"`
	Checks      []Check   `json:"checks"`
}

var (
	runIDPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{7,127}$`)
	gitSHAPattern = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
	uuidPattern   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	e164Pattern   = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
)

func main() {
	configPath := flag.String("config", "", "Twilio scenario JSON file")
	resultPath := flag.String("out", "", "immutable JSON evidence path")
	validateOnly := flag.Bool("validate-only", false, "validate the scenario without sending messages")
	flag.Parse()
	if *configPath == "" {
		fatal(2, "-config is required")
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fatal(2, err.Error())
	}
	if *validateOnly {
		fmt.Printf("Twilio scenario %q valid for %s\n", cfg.Name, cfg.Environment)
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
	// #nosec G304 -- the operator explicitly selects this local scenario file.
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read scenario: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse scenario: %w", err)
	}
	if value := strings.TrimSpace(os.Getenv("AURA_TWILIO_BASE_URL")); value != "" {
		cfg.BaseURL = value
	}
	cfg.RunID = strings.TrimSpace(os.Getenv("AURA_TWILIO_RUN_ID"))
	cfg.GitSHA = strings.TrimSpace(os.Getenv("AURA_TWILIO_GIT_SHA"))
	cfg.Tenant = strings.TrimSpace(os.Getenv("AURA_TWILIO_TENANT"))
	cfg.Token = strings.TrimSpace(os.Getenv("AURA_TWILIO_TOKEN"))
	cfg.RecipientID = strings.TrimSpace(os.Getenv("AURA_TWILIO_RECIPIENT_ID"))
	cfg.SMSNumber = strings.TrimSpace(os.Getenv("AURA_TWILIO_SMS_NUMBER"))
	cfg.WhatsAppNumber = strings.TrimSpace(os.Getenv("AURA_TWILIO_WHATSAPP_NUMBER"))
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validate(cfg Config) error {
	if cfg.Name != "auraedu-staging-messaging-providers" || cfg.Environment != "staging" || strings.TrimSpace(cfg.BaseURL) == "" {
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
	if err := validateTargetURL(cfg.BaseURL); err != nil {
		return err
	}
	if !runIDPattern.MatchString(cfg.RunID) || !gitSHAPattern.MatchString(cfg.GitSHA) {
		return errors.New("AURA_TWILIO_RUN_ID and AURA_TWILIO_GIT_SHA are required")
	}
	if cfg.Tenant == "" || len(cfg.Token) < 20 || strings.Contains(strings.ToLower(cfg.Token), "replace") || !uuidPattern.MatchString(cfg.RecipientID) {
		return errors.New("runtime tenant, bearer token and recipient UUID are required")
	}
	if !e164Pattern.MatchString(cfg.SMSNumber) || !e164Pattern.MatchString(cfg.WhatsAppNumber) {
		return errors.New("plain E.164 SMS and WhatsApp test numbers are required")
	}
	return nil
}

func validateTargetURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return errors.New("twilio proof requires a credential-free HTTPS origin")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasSuffix(host, ".example") {
		return errors.New("twilio proof cannot target a placeholder or loopback host")
	}
	return nil
}

func run(ctx context.Context, cfg Config) (Evidence, error) {
	evidence := Evidence{
		Name: cfg.Name, Environment: cfg.Environment, TargetURL: strings.TrimRight(cfg.BaseURL, "/"),
		RunID: cfg.RunID, GitSHA: cfg.GitSHA, StartedAt: time.Now().UTC(), Checks: make([]Check, 0, 6),
	}
	client := &http.Client{Timeout: cfg.Timeout.Duration}
	for _, delivery := range []struct{ channel, address string }{{"sms", cfg.SMSNumber}, {"whatsapp", cfg.WhatsAppNumber}} {
		checks, err := proveChannel(ctx, client, cfg, delivery.channel, delivery.address)
		evidence.Checks = append(evidence.Checks, checks...)
		if err != nil {
			return finishEvidence(evidence, err)
		}
	}
	return finishEvidence(evidence, nil)
}

func proveChannel(ctx context.Context, client *http.Client, cfg Config, channel, address string) ([]Check, error) {
	messageID, created := createMessage(ctx, client, cfg, channel, address)
	if !created {
		return failedChannelChecks(channel, "create"), errors.New("twilio proof failed during message creation")
	}
	sent, ok := sendMessage(ctx, client, cfg, messageID)
	if !ok || !validAcceptedRecord(sent, channel, messageID) {
		return failedChannelChecks(channel, "accepted"), errors.New("twilio provider acceptance was not persisted")
	}
	acceptedAt := time.Now().UTC()
	checks := []Check{passingCheck(channel+"-provider-accepted", acceptedAt, channel, messageID, "accepted", sent.SentAt.UTC().Format(time.RFC3339Nano))}
	delivered, ok := waitForDelivery(ctx, client, cfg, messageID, channel)
	if !ok {
		checks = append(checks, failedCheck(channel+"-delivered", "delivery"), failedCheck(channel+"-status-persisted", "status"))
		return checks, errors.New("twilio delivery feedback was not confirmed")
	}
	observedAt := time.Now().UTC()
	provider := *delivered.Provider
	status := *delivered.DeliveryStatus
	checks = append(checks,
		passingCheck(channel+"-delivered", observedAt, channel, messageID, "delivered", delivered.DeliveryStatusAt.UTC().Format(time.RFC3339Nano)),
		passingCheck(channel+"-status-persisted", observedAt, channel, messageID, provider, status),
	)
	return checks, nil
}

func createMessage(ctx context.Context, client *http.Client, cfg Config, channel, address string) (string, bool) {
	payload := map[string]any{
		"recipient_id": cfg.RecipientID,
		"channel":      channel,
		"subject":      "AuraEDU release delivery verification " + cfg.RunID,
		"body":         "AuraEDU staging notification delivery verification. No action is required.",
		"metadata":     map[string]any{"delivery_address": address, "release_probe": true, "run_id": cfg.RunID},
	}
	var record messageRecord
	if !doJSON(ctx, client, cfg, http.MethodPost, "/api/v1/messages", payload, http.StatusCreated, &record) {
		return "", false
	}
	return record.ID, uuidPattern.MatchString(record.ID) && record.Channel == channel && record.Status == "pending"
}

func sendMessage(ctx context.Context, client *http.Client, cfg Config, messageID string) (messageRecord, bool) {
	var record messageRecord
	ok := doJSON(ctx, client, cfg, http.MethodPost, "/api/v1/messages/"+url.PathEscape(messageID)+"/send", nil, http.StatusOK, &record)
	return record, ok
}

func getMessage(ctx context.Context, client *http.Client, cfg Config, messageID string) (messageRecord, bool) {
	var record messageRecord
	ok := doJSON(ctx, client, cfg, http.MethodGet, "/api/v1/messages/"+url.PathEscape(messageID), nil, http.StatusOK, &record)
	return record, ok
}

func waitForDelivery(ctx context.Context, client *http.Client, cfg Config, messageID, channel string) (messageRecord, bool) {
	deadline := time.NewTimer(cfg.DeliveryTimeout.Duration)
	defer deadline.Stop()
	for {
		record, ok := getMessage(ctx, client, cfg, messageID)
		if ok && validDeliveredRecord(record, channel, messageID) {
			return record, true
		}
		if ok && record.DeliveryStatus != nil && *record.DeliveryStatus == "failed" {
			return record, false
		}
		wait := time.NewTimer(cfg.PollInterval.Duration)
		select {
		case <-ctx.Done():
			stopTimer(wait)
			return record, false
		case <-deadline.C:
			stopTimer(wait)
			return record, false
		case <-wait.C:
		}
	}
}

func validAcceptedRecord(record messageRecord, channel, messageID string) bool {
	if record.ID != messageID || record.Channel != channel || record.Status != "sent" || record.Provider == nil || *record.Provider != "twilio" ||
		record.SentAt == nil || record.DeliveryStatus == nil || record.DeliveryStatusAt == nil {
		return false
	}
	return *record.DeliveryStatus == "accepted" || *record.DeliveryStatus == "delivered"
}

func validDeliveredRecord(record messageRecord, channel, messageID string) bool {
	return validAcceptedRecord(record, channel, messageID) && *record.DeliveryStatus == "delivered"
}

func doJSON(ctx context.Context, client *http.Client, cfg Config, method, path string, payload any, expectedStatus int, target any) bool {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return false
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(cfg.BaseURL, "/")+path, body)
	if err != nil {
		return false
	}
	request.Header.Set("Authorization", "Bearer "+cfg.Token)
	request.Header.Set("X-Tenant-Code", cfg.Tenant)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	limited, readErr := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || len(limited) > 64<<10 || response.StatusCode != expectedStatus {
		return false
	}
	return json.Unmarshal(limited, target) == nil
}

func passingCheck(name string, observedAt time.Time, values ...any) Check {
	return Check{Name: name, Passed: true, ObservedAt: observedAt, EvidenceFingerprint: fingerprint(fmt.Sprint(values...))}
}

func failedCheck(name, stage string) Check {
	return Check{Name: name, Passed: false, ObservedAt: time.Now().UTC(), EvidenceFingerprint: fingerprint(name + ":" + stage)}
}

func failedChannelChecks(channel, stage string) []Check {
	return []Check{
		failedCheck(channel+"-provider-accepted", stage),
		failedCheck(channel+"-delivered", stage),
		failedCheck(channel+"-status-persisted", stage),
	}
}

func finishEvidence(evidence Evidence, runErr error) (Evidence, error) {
	evidence.FinishedAt = time.Now().UTC()
	evidence.AllPassed = runErr == nil && len(evidence.Checks) == 6
	for _, check := range evidence.Checks {
		evidence.AllPassed = evidence.AllPassed && check.Passed
	}
	return evidence, runErr
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func fingerprint(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])[:16]
}

func writeEvidence(path string, data []byte) error {
	// #nosec G304 -- the operator explicitly selects this exclusive evidence path.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return errors.Join(err, file.Close())
	}
	return file.Close()
}
