package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRegion      = "eu-central-1"
	defaultPrefix      = "auraedu"
	defaultRetention   = 35
	minimumRetention   = 30
	defaultJobTimeout  = 4 * time.Hour
	defaultHTTPTimeout = 2 * time.Minute
)

type config struct {
	NATSURL        string
	NATSCreds      string
	NATSCLI        string
	S3Endpoint     *url.URL
	S3Region       string
	S3Bucket       string
	S3Prefix       string
	S3AccessKey    string
	S3SecretKey    string
	S3SessionToken string
	RetentionDays  int
	HeartbeatURL   *url.URL
	HeartbeatToken string
	AlertURL       *url.URL
	AlertToken     string
	JobTimeout     time.Duration
	HTTPTimeout    time.Duration
}

type commandRunner interface {
	Run(context.Context, string, ...string) error
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("backup command failed: %w", err)
	}
	return nil
}

type backupResult struct {
	ObjectKey   string    `json:"object_key"`
	SHA256      string    `json:"sha256"`
	Bytes       int64     `json:"bytes"`
	CompletedAt time.Time `json:"completed_at"`
}

func main() {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.JobTimeout)
	defer cancel()
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	result, err := executeBackup(ctx, cfg, execRunner{}, client, time.Now().UTC(), os.TempDir())
	if err != nil {
		alertErr := sendStatus(context.Background(), client, cfg.AlertURL, cfg.AlertToken, map[string]any{
			"service": "nats-backup",
			"status":  "failed",
			"error":   publicError(err),
		})
		if alertErr != nil {
			log.Printf("backup failed and alert delivery failed: %v", alertErr)
		}
		log.Fatalf("backup failed: %v", err)
	}

	payload := map[string]any{
		"service":      "nats-backup",
		"status":       "ok",
		"object_key":   result.ObjectKey,
		"sha256":       result.SHA256,
		"bytes":        result.Bytes,
		"completed_at": result.CompletedAt.Format(time.RFC3339),
	}
	statusCtx, statusCancel := context.WithTimeout(context.Background(), cfg.HTTPTimeout)
	defer statusCancel()
	if err := sendStatus(statusCtx, client, cfg.HeartbeatURL, cfg.HeartbeatToken, payload); err != nil {
		alertErr := sendStatus(context.Background(), client, cfg.AlertURL, cfg.AlertToken, map[string]any{
			"service": "nats-backup",
			"status":  "failed",
			"error":   "backup stored but success heartbeat failed",
		})
		if alertErr != nil {
			log.Printf("heartbeat and alert delivery failed: %v", alertErr)
		}
		log.Fatalf("success heartbeat failed: %v", err)
	}

	encoded, _ := json.Marshal(result)
	log.Printf("backup complete %s", encoded)
}

func loadConfig(getenv func(string) string) (config, error) {
	required := func(key string) (string, error) {
		value := strings.TrimSpace(getenv(key))
		if value == "" {
			return "", fmt.Errorf("%s is required", key)
		}
		return value, nil
	}

	natsURL, err := required("NATS_URL")
	if err != nil {
		return config{}, err
	}
	if !strings.Contains(natsURL, "://") {
		natsURL = "nats://" + natsURL
	}
	parsedNATS, err := url.Parse(natsURL)
	if err != nil || parsedNATS.Host == "" || (parsedNATS.Scheme != "nats" && parsedNATS.Scheme != "tls") || parsedNATS.User != nil || parsedNATS.RawQuery != "" || parsedNATS.Fragment != "" {
		return config{}, errors.New("NATS_URL must be a nats:// or tls:// endpoint without credentials, query or fragment")
	}

	endpoint, err := requiredHTTPSURL("DR_BACKUP_S3_ENDPOINT", getenv)
	if err != nil {
		return config{}, err
	}
	heartbeat, err := requiredHTTPSURL("DR_BACKUP_HEARTBEAT_URL", getenv)
	if err != nil {
		return config{}, err
	}
	alert, err := requiredHTTPSURL("DR_BACKUP_ALERT_URL", getenv)
	if err != nil {
		return config{}, err
	}

	bucket, err := required("DR_BACKUP_S3_BUCKET")
	if err != nil {
		return config{}, err
	}
	if !validBucket(bucket) {
		return config{}, errors.New("DR_BACKUP_S3_BUCKET must be a valid DNS-style bucket name")
	}
	accessKey, err := required("DR_BACKUP_S3_ACCESS_KEY_ID")
	if err != nil {
		return config{}, err
	}
	secretKey, err := required("DR_BACKUP_S3_SECRET_ACCESS_KEY")
	if err != nil {
		return config{}, err
	}
	heartbeatToken, err := required("DR_BACKUP_HEARTBEAT_TOKEN")
	if err != nil {
		return config{}, err
	}
	alertToken, err := required("DR_BACKUP_ALERT_TOKEN")
	if err != nil {
		return config{}, err
	}

	retention, err := integerEnv(getenv, "DR_BACKUP_RETENTION_DAYS", defaultRetention)
	if err != nil || retention < minimumRetention {
		return config{}, fmt.Errorf("DR_BACKUP_RETENTION_DAYS must be at least %d", minimumRetention)
	}
	jobTimeout, err := durationEnv(getenv, "DR_BACKUP_JOB_TIMEOUT", defaultJobTimeout)
	if err != nil || jobTimeout <= 0 || jobTimeout > 12*time.Hour {
		return config{}, errors.New("DR_BACKUP_JOB_TIMEOUT must be greater than zero and at most 12h")
	}
	httpTimeout, err := durationEnv(getenv, "DR_BACKUP_HTTP_TIMEOUT", defaultHTTPTimeout)
	if err != nil || httpTimeout <= 0 || httpTimeout > 10*time.Minute {
		return config{}, errors.New("DR_BACKUP_HTTP_TIMEOUT must be greater than zero and at most 10m")
	}

	region := strings.TrimSpace(getenv("DR_BACKUP_S3_REGION"))
	if region == "" {
		region = defaultRegion
	}
	prefix := strings.Trim(strings.TrimSpace(getenv("DR_BACKUP_S3_PREFIX")), "/")
	if prefix == "" {
		prefix = defaultPrefix
	}
	if !validPrefix(prefix) {
		return config{}, errors.New("DR_BACKUP_S3_PREFIX contains unsafe path segments")
	}
	natsCLI := strings.TrimSpace(getenv("NATS_CLI_PATH"))
	if natsCLI == "" {
		natsCLI = "/nats"
	}

	return config{
		NATSURL:        parsedNATS.String(),
		NATSCreds:      strings.TrimSpace(getenv("NATS_CREDS")),
		NATSCLI:        natsCLI,
		S3Endpoint:     endpoint,
		S3Region:       region,
		S3Bucket:       bucket,
		S3Prefix:       prefix,
		S3AccessKey:    accessKey,
		S3SecretKey:    secretKey,
		S3SessionToken: strings.TrimSpace(getenv("DR_BACKUP_S3_SESSION_TOKEN")),
		RetentionDays:  retention,
		HeartbeatURL:   heartbeat,
		HeartbeatToken: heartbeatToken,
		AlertURL:       alert,
		AlertToken:     alertToken,
		JobTimeout:     jobTimeout,
		HTTPTimeout:    httpTimeout,
	}, nil
}

func validBucket(value string) bool {
	if len(value) < 3 || len(value) > 63 || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") || strings.Contains(value, "..") {
		return false
	}
	for index, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || (character == '-' && index > 0 && index < len(value)-1) || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validPrefix(value string) bool {
	if value == "" || strings.Contains(value, "\\") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		for _, character := range segment {
			if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || strings.ContainsRune("-_.", character) {
				continue
			}
			return false
		}
	}
	return true
}

func requiredHTTPSURL(key string, getenv func(string) string) (*url.URL, error) {
	value := strings.TrimSpace(getenv(key))
	parsed, err := url.Parse(value)
	if value == "" || err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("%s must be an HTTPS URL without credentials, query or fragment", key)
	}
	return parsed, nil
}

func integerEnv(getenv func(string) string, key string, fallback int) (int, error) {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

func durationEnv(getenv func(string) string, key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return fallback, nil
	}
	return time.ParseDuration(value)
}

func executeBackup(ctx context.Context, cfg config, runner commandRunner, client *http.Client, now time.Time, tempRoot string) (backupResult, error) {
	working, err := os.MkdirTemp(tempRoot, "auraedu-nats-backup-")
	if err != nil {
		return backupResult{}, fmt.Errorf("create backup workspace: %w", err)
	}
	defer os.RemoveAll(working)

	backupDir := filepath.Join(working, "account")
	args := []string{"--server", cfg.NATSURL}
	if cfg.NATSCreds != "" {
		args = append(args, "--creds", cfg.NATSCreds)
	}
	args = append(args, "account", "backup", backupDir, "--consumers", "--check", "--critical-warnings", "--force")
	if err := runner.Run(ctx, cfg.NATSCLI, args...); err != nil {
		return backupResult{}, err
	}
	if err := requireBackupPayload(backupDir); err != nil {
		return backupResult{}, err
	}

	manifest := map[string]any{
		"format":         "nats-account-backup",
		"created_at":     now.Format(time.RFC3339),
		"retention_days": cfg.RetentionDays,
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(backupDir, "auraedu-manifest.json"), manifestBytes, 0o600); err != nil {
		return backupResult{}, fmt.Errorf("write backup manifest: %w", err)
	}

	archivePath := filepath.Join(working, "nats-account.tar.gz")
	if err := createArchive(backupDir, archivePath); err != nil {
		return backupResult{}, err
	}
	digest, size, err := fileDigest(archivePath)
	if err != nil {
		return backupResult{}, err
	}
	key := path.Join(cfg.S3Prefix, "nats", now.Format("2006/01/02"), fmt.Sprintf("nats-account-%s.tar.gz", now.Format("20060102T150405Z")))
	if err := uploadS3(ctx, client, cfg, key, archivePath, digest, size, now); err != nil {
		return backupResult{}, err
	}
	return backupResult{ObjectKey: key, SHA256: digest, Bytes: size, CompletedAt: time.Now().UTC()}, nil
}

func requireBackupPayload(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("inspect account backup: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return nil
		}
	}
	return errors.New("account backup contains no stream directories")
}

func createArchive(sourceDir, target string) error {
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create backup archive: %w", err)
	}
	gzipWriter := gzip.NewWriter(output)
	tarWriter := tar.NewWriter(gzipWriter)

	err = filepath.WalkDir(sourceDir, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceDir, filePath)
		if err != nil || relative == "." {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		header.Uid, header.Gid = 0, 0
		header.Uname, header.Gname = "", ""
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		input, err := os.Open(filePath)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tarWriter, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		_ = output.Close()
		return fmt.Errorf("archive account backup: %w", err)
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		_ = output.Close()
		return fmt.Errorf("finalize backup archive: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		_ = output.Close()
		return fmt.Errorf("compress backup archive: %w", err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close backup archive: %w", err)
	}
	return nil
}

func fileDigest(filePath string) (string, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("open backup archive: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, fmt.Errorf("hash backup archive: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func uploadS3(ctx context.Context, client *http.Client, cfg config, key, filePath, digest string, size int64, now time.Time) error {
	body, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open archive for upload: %w", err)
	}
	defer body.Close()

	target := *cfg.S3Endpoint
	target.Path = path.Join(target.Path, cfg.S3Bucket, key)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, target.String(), body)
	if err != nil {
		return fmt.Errorf("create object-store request: %w", err)
	}
	request.ContentLength = size
	request.Header.Set("Content-Type", "application/gzip")
	request.Header.Set("If-None-Match", "*")
	request.Header.Set("X-Amz-Content-Sha256", digest)
	request.Header.Set("X-Amz-Date", now.Format("20060102T150405Z"))
	request.Header.Set("X-Amz-Server-Side-Encryption", "AES256")
	request.Header.Set("X-Amz-Object-Lock-Mode", "COMPLIANCE")
	request.Header.Set("X-Amz-Object-Lock-Retain-Until-Date", now.AddDate(0, 0, cfg.RetentionDays).Format(time.RFC3339))
	if cfg.S3SessionToken != "" {
		request.Header.Set("X-Amz-Security-Token", cfg.S3SessionToken)
	}
	signV4(request, cfg, digest, now)

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("upload encrypted immutable backup: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("object store rejected backup with status %d", response.StatusCode)
	}
	return nil
}

func signV4(request *http.Request, cfg config, payloadHash string, now time.Time) {
	request.Header.Set("Host", request.URL.Host)
	headerNames := make([]string, 0, len(request.Header))
	for name := range request.Header {
		headerNames = append(headerNames, strings.ToLower(name))
	}
	sort.Strings(headerNames)
	canonicalHeaders := strings.Builder{}
	for _, name := range headerNames {
		values := request.Header.Values(name)
		for index := range values {
			values[index] = strings.Join(strings.Fields(values[index]), " ")
		}
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(strings.Join(values, ","))
		canonicalHeaders.WriteByte('\n')
	}
	signedHeaders := strings.Join(headerNames, ";")
	canonicalRequest := strings.Join([]string{
		request.Method,
		request.URL.EscapedPath(),
		request.URL.Query().Encode(),
		canonicalHeaders.String(),
		signedHeaders,
		payloadHash,
	}, "\n")
	date := now.Format("20060102")
	scope := date + "/" + cfg.S3Region + "/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + now.Format("20060102T150405Z") + "\n" + scope + "\n" + hashHex([]byte(canonicalRequest))
	dateKey := hmacBytes([]byte("AWS4"+cfg.S3SecretKey), []byte(date))
	regionKey := hmacBytes(dateKey, []byte(cfg.S3Region))
	serviceKey := hmacBytes(regionKey, []byte("s3"))
	signingKey := hmacBytes(serviceKey, []byte("aws4_request"))
	signature := hex.EncodeToString(hmacBytes(signingKey, []byte(stringToSign)))
	request.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+cfg.S3AccessKey+"/"+scope+",SignedHeaders="+signedHeaders+",Signature="+signature)
}

func hashHex(value []byte) string {
	hash := sha256.Sum256(value)
	return hex.EncodeToString(hash[:])
}

func hmacBytes(key, value []byte) []byte {
	hash := hmac.New(sha256.New, key)
	_, _ = hash.Write(value)
	return hash.Sum(nil)
}

func sendStatus(ctx context.Context, client *http.Client, endpoint *url.URL, token string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("status endpoint returned %d", response.StatusCode)
	}
	return nil
}

func publicError(err error) string {
	message := err.Error()
	if len(message) > 240 {
		message = message[:240]
	}
	return message
}
