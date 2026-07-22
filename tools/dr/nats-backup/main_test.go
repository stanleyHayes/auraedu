package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	args []string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func (runner *fakeRunner) Run(_ context.Context, _ string, args ...string) error {
	runner.args = append([]string(nil), args...)
	backupDir := args[len(args)-5]
	streamDir := filepath.Join(backupDir, "AURA_EVENTS")
	if err := os.MkdirAll(streamDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(streamDir, "stream.json"), []byte(`{"name":"AURA_EVENTS"}`), 0o600)
}

func TestLoadConfigFailsClosed(t *testing.T) {
	values := validEnvironment()
	delete(values, "DR_BACKUP_ALERT_TOKEN")
	_, err := loadConfig(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "DR_BACKUP_ALERT_TOKEN") {
		t.Fatalf("expected missing alert token error, got %v", err)
	}

	values = validEnvironment()
	values["DR_BACKUP_S3_ENDPOINT"] = "http://object-store.invalid"
	_, err = loadConfig(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "HTTPS URL") {
		t.Fatalf("expected HTTPS validation error, got %v", err)
	}

	values = validEnvironment()
	values["DR_BACKUP_RETENTION_DAYS"] = "7"
	_, err = loadConfig(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "at least 30") {
		t.Fatalf("expected retention validation error, got %v", err)
	}

	values = validEnvironment()
	values["NATS_URL"] = "nats://operator:secret@nats:4222"
	_, err = loadConfig(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "without credentials") {
		t.Fatalf("expected credential-bearing NATS URL rejection, got %v", err)
	}

	values = validEnvironment()
	values["DR_BACKUP_S3_PREFIX"] = "auraedu/../escape"
	_, err = loadConfig(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "unsafe path") {
		t.Fatalf("expected unsafe object prefix rejection, got %v", err)
	}

	values = validEnvironment()
	values["DR_BACKUP_S3_BUCKET"] = "Invalid_Bucket"
	_, err = loadConfig(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "DNS-style") {
		t.Fatalf("expected invalid bucket rejection, got %v", err)
	}
}

func TestExecuteBackupCreatesImmutableEncryptedObject(t *testing.T) {
	var uploaded []byte
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPut {
			t.Fatalf("unexpected method %s", request.Method)
		}
		if request.URL.Path != "/dr-bucket/auraedu/nats/2026/07/20/nats-account-20260720T043500Z.tar.gz" {
			t.Fatalf("unexpected object path %s", request.URL.Path)
		}
		for name, want := range map[string]string{
			"If-None-Match":                       "*",
			"X-Amz-Server-Side-Encryption":        "AES256",
			"X-Amz-Object-Lock-Mode":              "COMPLIANCE",
			"X-Amz-Object-Lock-Retain-Until-Date": "2026-08-24T04:35:00Z",
		} {
			if got := request.Header.Get(name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		if !strings.Contains(request.Header.Get("Authorization"), "Credential=access-key/20260720/eu-central-1/s3/aws4_request") {
			t.Errorf("unexpected authorization %q", request.Header.Get("Authorization"))
		}
		uploaded, _ = io.ReadAll(request.Body)
		return testResponse(http.StatusOK), nil
	})}

	endpoint, _ := url.Parse("https://objects.example")
	client.Timeout = time.Second
	runner := &fakeRunner{}
	now := time.Date(2026, 7, 20, 4, 35, 0, 0, time.UTC)
	cfg := config{
		NATSURL:       "nats://nats:4222",
		NATSCLI:       "/nats",
		S3Endpoint:    endpoint,
		S3Region:      "eu-central-1",
		S3Bucket:      "dr-bucket",
		S3Prefix:      "auraedu",
		S3AccessKey:   "access-key",
		S3SecretKey:   "secret-key",
		RetentionDays: 35,
	}
	result, err := executeBackup(context.Background(), cfg, runner, client, now, t.TempDir())
	if err != nil {
		t.Fatalf("execute backup: %v", err)
	}
	if result.Bytes == 0 || result.SHA256 == "" || len(uploaded) == 0 {
		t.Fatalf("incomplete result: %+v, uploaded=%d", result, len(uploaded))
	}
	joined := strings.Join(runner.args, " ")
	for _, required := range []string{"account backup", "--consumers", "--check", "--critical-warnings", "--force"} {
		if !strings.Contains(joined, required) {
			t.Errorf("backup args %q missing %q", joined, required)
		}
	}
	assertArchiveContains(t, uploaded, "AURA_EVENTS/stream.json", "auraedu-manifest.json")
}

func TestSendStatusUsesBearerAuthAndRejectsFailure(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer monitor-token" {
			t.Errorf("missing bearer auth")
		}
		return testResponse(http.StatusAccepted), nil
	})}
	endpoint, _ := url.Parse("https://monitor.example/heartbeat")
	if err := sendStatus(context.Background(), client, endpoint, "monitor-token", map[string]string{"status": "ok"}); err != nil {
		t.Fatalf("send status: %v", err)
	}

	failing := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return testResponse(http.StatusBadGateway), nil
	})}
	if err := sendStatus(context.Background(), failing, endpoint, "monitor-token", nil); err == nil {
		t.Fatal("expected non-2xx status endpoint to fail")
	}
}

func testResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}
}

func TestSignV4IncludesSessionToken(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPut, "https://objects.example/dr/key", nil)
	request.Header.Set("X-Amz-Date", "20260720T043500Z")
	request.Header.Set("X-Amz-Content-Sha256", strings.Repeat("a", 64))
	request.Header.Set("X-Amz-Security-Token", "session-token")
	cfg := config{S3Region: "eu-central-1", S3AccessKey: "access", S3SecretKey: "secret"}
	signV4(request, cfg, strings.Repeat("a", 64), time.Date(2026, 7, 20, 4, 35, 0, 0, time.UTC))
	if !strings.Contains(request.Header.Get("Authorization"), "x-amz-security-token") {
		t.Fatalf("session token was not covered by signature: %s", request.Header.Get("Authorization"))
	}
}

func TestSignV4MatchesAWSPutObjectReference(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPut, "https://examplebucket.s3.amazonaws.com/test%24file.text", nil)
	request.Header.Set("Date", "Fri, 24 May 2013 00:00:00 GMT")
	request.Header.Set("X-Amz-Date", "20130524T000000Z")
	request.Header.Set("X-Amz-Storage-Class", "REDUCED_REDUNDANCY")
	payloadHash := "44ce7dd67c959e0d3524ffac1771dfbba87d2b6b4b4e99e42034a8b803f8b072"
	request.Header.Set("X-Amz-Content-Sha256", payloadHash)
	cfg := config{
		S3Region:    "us-east-1",
		S3AccessKey: "unit-test-access",
		S3SecretKey: "unit-test-secret",
	}
	signV4(request, cfg, payloadHash, time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC))
	want := "AWS4-HMAC-SHA256 Credential=unit-test-access/20130524/us-east-1/s3/aws4_request,SignedHeaders=date;host;x-amz-content-sha256;x-amz-date;x-amz-storage-class,Signature=1171458f713cadceefd2d9a6911c804236f3e7e50f99757a2b10b2c6cedb2df8"
	if got := request.Header.Get("Authorization"); got != want {
		t.Fatalf("AWS SigV4 reference mismatch\n got: %s\nwant: %s", got, want)
	}
}

func validEnvironment() map[string]string {
	return map[string]string{
		"NATS_URL":                       "nats:4222",
		"DR_BACKUP_S3_ENDPOINT":          "https://s3.eu-central-1.amazonaws.com",
		"DR_BACKUP_S3_BUCKET":            "auraedu-dr",
		"DR_BACKUP_S3_ACCESS_KEY_ID":     "access",
		"DR_BACKUP_S3_SECRET_ACCESS_KEY": "secret",
		"DR_BACKUP_HEARTBEAT_URL":        "https://monitor.example/heartbeat",
		"DR_BACKUP_HEARTBEAT_TOKEN":      "heartbeat-token",
		"DR_BACKUP_ALERT_URL":            "https://monitor.example/alerts",
		"DR_BACKUP_ALERT_TOKEN":          "alert-token",
	}
}

func assertArchiveContains(t *testing.T, content []byte, names ...string) {
	t.Helper()
	gzipReader, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("open gzip: %v", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	found := map[string]bool{}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		found[header.Name] = true
	}
	for _, name := range names {
		if !found[name] {
			t.Errorf("archive missing %s", name)
		}
	}
}
