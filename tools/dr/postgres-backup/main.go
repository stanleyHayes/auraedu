package main

import (
	"bytes"
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
	defaultJobTimeout  = 55 * time.Minute
	defaultHTTPTimeout = 2 * time.Minute
)

type database struct {
	Name string
	URL  string
}

type config struct {
	Databases      []database
	PGDump         string
	PGRestore      string
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
	Run(context.Context, []string, string, ...string) error
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, environment []string, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Env = mergeEnvironment(os.Environ(), environment)
	command.Stdout = io.Discard
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("backup command failed: %w", err)
	}
	return nil
}

func mergeEnvironment(base, overrides []string) []string {
	keys := make(map[string]struct{}, len(overrides))
	for _, value := range overrides {
		key, _, found := strings.Cut(value, "=")
		if found {
			keys[key] = struct{}{}
		}
	}
	merged := make([]string, 0, len(base)+len(overrides))
	for _, value := range base {
		key, _, found := strings.Cut(value, "=")
		if _, overridden := keys[key]; found && overridden {
			continue
		}
		merged = append(merged, value)
	}
	return append(merged, overrides...)
}

type backupObject struct {
	Database    string    `json:"database"`
	ObjectKey   string    `json:"object_key"`
	SHA256      string    `json:"sha256"`
	Bytes       int64     `json:"bytes"`
	CompletedAt time.Time `json:"completed_at"`
}

type backupResult struct {
	Objects     []backupObject `json:"objects"`
	TotalBytes  int64          `json:"total_bytes"`
	CompletedAt time.Time      `json:"completed_at"`
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
			"service": "postgres-backup",
			"status":  "failed",
			"error":   publicError(err),
		})
		if alertErr != nil {
			log.Printf("backup failed and alert delivery failed: %v", alertErr)
		}
		log.Fatalf("backup failed: %v", err)
	}

	payload := map[string]any{
		"service":        "postgres-backup",
		"status":         "ok",
		"database_count": len(result.Objects),
		"total_bytes":    result.TotalBytes,
		"completed_at":   result.CompletedAt.Format(time.RFC3339),
		"objects":        result.Objects,
	}
	statusCtx, statusCancel := context.WithTimeout(context.Background(), cfg.HTTPTimeout)
	defer statusCancel()
	if err := sendStatus(statusCtx, client, cfg.HeartbeatURL, cfg.HeartbeatToken, payload); err != nil {
		alertErr := sendStatus(context.Background(), client, cfg.AlertURL, cfg.AlertToken, map[string]any{
			"service": "postgres-backup",
			"status":  "failed",
			"error":   "all exports stored but success heartbeat failed",
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

	databaseList, err := required("POSTGRES_DATABASES")
	if err != nil {
		return config{}, err
	}
	databases, err := parseDatabases(databaseList, getenv)
	if err != nil {
		return config{}, err
	}
	endpoint, err := requiredHTTPSURL("DR_BACKUP_S3_ENDPOINT", getenv)
	if err != nil {
		return config{}, err
	}
	heartbeat, err := requiredHTTPSURL("DR_POSTGRES_BACKUP_HEARTBEAT_URL", getenv)
	if err != nil {
		return config{}, err
	}
	alert, err := requiredHTTPSURL("DR_POSTGRES_BACKUP_ALERT_URL", getenv)
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
	heartbeatToken, err := required("DR_POSTGRES_BACKUP_HEARTBEAT_TOKEN")
	if err != nil {
		return config{}, err
	}
	alertToken, err := required("DR_POSTGRES_BACKUP_ALERT_TOKEN")
	if err != nil {
		return config{}, err
	}

	retention, err := integerEnv(getenv, "DR_BACKUP_RETENTION_DAYS", defaultRetention)
	if err != nil || retention < minimumRetention {
		return config{}, fmt.Errorf("DR_BACKUP_RETENTION_DAYS must be at least %d", minimumRetention)
	}
	jobTimeout, err := durationEnv(getenv, "DR_POSTGRES_BACKUP_JOB_TIMEOUT", defaultJobTimeout)
	if err != nil || jobTimeout <= 0 || jobTimeout > time.Hour {
		return config{}, errors.New("DR_POSTGRES_BACKUP_JOB_TIMEOUT must be greater than zero and at most 1h")
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
	pgDump := strings.TrimSpace(getenv("PG_DUMP_PATH"))
	if pgDump == "" {
		pgDump = "/usr/local/bin/pg_dump"
	}
	pgRestore := strings.TrimSpace(getenv("PG_RESTORE_PATH"))
	if pgRestore == "" {
		pgRestore = "/usr/local/bin/pg_restore"
	}

	return config{
		Databases:      databases,
		PGDump:         pgDump,
		PGRestore:      pgRestore,
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

func parseDatabases(list string, getenv func(string) string) ([]database, error) {
	names := strings.Split(list, ",")
	databases := make([]database, 0, len(names))
	seen := map[string]struct{}{}
	for _, rawName := range names {
		name := strings.TrimSpace(rawName)
		if !validDatabaseName(name) {
			return nil, fmt.Errorf("POSTGRES_DATABASES contains invalid database name %q", name)
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("POSTGRES_DATABASES contains duplicate database %q", name)
		}
		seen[name] = struct{}{}
		envKey := "POSTGRES_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_DATABASE_URL"
		connection := strings.TrimSpace(getenv(envKey))
		parsed, err := url.Parse(connection)
		if connection == "" || err != nil || parsed.Host == "" || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Fragment != "" {
			return nil, fmt.Errorf("%s must be a postgres:// or postgresql:// URL without a fragment", envKey)
		}
		if _, err := libpqEnvironment(connection); err != nil {
			return nil, fmt.Errorf("%s is invalid: %w", envKey, err)
		}
		databases = append(databases, database{Name: name, URL: connection})
	}
	if len(databases) == 0 {
		return nil, errors.New("POSTGRES_DATABASES must contain at least one database")
	}
	return databases, nil
}

func libpqEnvironment(connection string) ([]string, error) {
	parsed, err := url.Parse(connection)
	if err != nil || parsed.Hostname() == "" {
		return nil, errors.New("connection URL must include a host")
	}
	if parsed.User == nil || parsed.User.Username() == "" {
		return nil, errors.New("connection URL must include a user")
	}
	databaseName, err := url.PathUnescape(strings.TrimPrefix(parsed.EscapedPath(), "/"))
	if err != nil || databaseName == "" || strings.Contains(databaseName, "/") {
		return nil, errors.New("connection URL must include one database path segment")
	}
	port := parsed.Port()
	if port == "" {
		port = "5432"
	}
	environment := []string{
		"PGHOST=" + parsed.Hostname(),
		"PGPORT=" + port,
		"PGUSER=" + parsed.User.Username(),
		"PGDATABASE=" + databaseName,
		"PGAPPNAME=auraedu-postgres-backup",
	}
	if password, present := parsed.User.Password(); present {
		environment = append(environment, "PGPASSWORD="+password)
	}
	queryMapping := map[string]string{
		"sslmode":              "PGSSLMODE",
		"sslrootcert":          "PGSSLROOTCERT",
		"sslcert":              "PGSSLCERT",
		"sslkey":               "PGSSLKEY",
		"connect_timeout":      "PGCONNECT_TIMEOUT",
		"target_session_attrs": "PGTARGETSESSIONATTRS",
	}
	for key, values := range parsed.Query() {
		environmentKey, allowed := queryMapping[key]
		if !allowed || len(values) != 1 || values[0] == "" {
			return nil, fmt.Errorf("connection URL contains unsupported parameter %q", key)
		}
		environment = append(environment, environmentKey+"="+values[0])
	}
	return environment, nil
}

func validDatabaseName(value string) bool {
	if len(value) < 2 || len(value) > 50 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' {
			continue
		}
		return false
	}
	return !strings.HasSuffix(value, "-")
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

func executeBackup(ctx context.Context, cfg config, runner commandRunner, client *http.Client, now time.Time, tempRoot string) (backupResult, error) {
	working, err := os.MkdirTemp(tempRoot, "auraedu-postgres-backup-")
	if err != nil {
		return backupResult{}, fmt.Errorf("create backup workspace: %w", err)
	}
	defer os.RemoveAll(working)

	result := backupResult{Objects: make([]backupObject, 0, len(cfg.Databases))}
	for _, item := range cfg.Databases {
		if err := ctx.Err(); err != nil {
			return backupResult{}, err
		}
		dumpPath := filepath.Join(working, item.Name+".dump")
		environment, err := libpqEnvironment(item.URL)
		if err != nil {
			return backupResult{}, fmt.Errorf("prepare %s connection: %w", item.Name, err)
		}
		if err := runner.Run(ctx, environment, cfg.PGDump,
			"--file", dumpPath,
			"--format=custom",
			"--no-owner",
			"--no-privileges",
		); err != nil {
			return backupResult{}, fmt.Errorf("export %s: %w", item.Name, err)
		}
		info, err := os.Stat(dumpPath)
		if err != nil || info.Size() == 0 {
			return backupResult{}, fmt.Errorf("export %s produced no backup payload", item.Name)
		}
		if err := runner.Run(ctx, nil, cfg.PGRestore, "--list", dumpPath); err != nil {
			return backupResult{}, fmt.Errorf("validate %s export catalogue: %w", item.Name, err)
		}
		digest, size, err := fileDigest(dumpPath)
		if err != nil {
			return backupResult{}, err
		}
		key := path.Join(cfg.S3Prefix, "postgres", now.Format("2006/01/02"), fmt.Sprintf("%s-%s.dump", item.Name, now.Format("20060102T150405Z")))
		if err := uploadS3(ctx, client, cfg, item.Name, key, dumpPath, digest, size, now); err != nil {
			return backupResult{}, fmt.Errorf("store %s export: %w", item.Name, err)
		}
		completedAt := time.Now().UTC()
		result.Objects = append(result.Objects, backupObject{
			Database: item.Name, ObjectKey: key, SHA256: digest, Bytes: size, CompletedAt: completedAt,
		})
		result.TotalBytes += size
	}
	result.CompletedAt = time.Now().UTC()
	return result, nil
}

func fileDigest(filePath string) (string, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("open database export: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, fmt.Errorf("hash database export: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func uploadS3(ctx context.Context, client *http.Client, cfg config, databaseName, key, filePath, digest string, size int64, now time.Time) error {
	body, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open database export for upload: %w", err)
	}
	defer body.Close()

	target := *cfg.S3Endpoint
	target.Path = path.Join(target.Path, cfg.S3Bucket, key)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, target.String(), body)
	if err != nil {
		return fmt.Errorf("create object-store request: %w", err)
	}
	request.ContentLength = size
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("If-None-Match", "*")
	request.Header.Set("X-Amz-Content-Sha256", digest)
	request.Header.Set("X-Amz-Date", now.Format("20060102T150405Z"))
	request.Header.Set("X-Amz-Meta-Auraedu-Database", databaseName)
	request.Header.Set("X-Amz-Meta-Auraedu-Pg-Major", "18")
	request.Header.Set("X-Amz-Meta-Auraedu-Sha256", digest)
	request.Header.Set("X-Amz-Server-Side-Encryption", "AES256")
	request.Header.Set("X-Amz-Object-Lock-Mode", "COMPLIANCE")
	request.Header.Set("X-Amz-Object-Lock-Retain-Until-Date", now.AddDate(0, 0, cfg.RetentionDays).Format(time.RFC3339))
	if cfg.S3SessionToken != "" {
		request.Header.Set("X-Amz-Security-Token", cfg.S3SessionToken)
	}
	signV4(request, cfg, digest, now)

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("upload encrypted immutable database export: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("object store rejected database export with status %d", response.StatusCode)
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
