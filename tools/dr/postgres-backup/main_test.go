package main

import (
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
	calls []runnerCall
}

type runnerCall struct {
	environment []string
	name        string
	args        []string
}

func (runner *fakeRunner) Run(_ context.Context, environment []string, name string, args ...string) error {
	runner.calls = append(runner.calls, runnerCall{
		environment: append([]string(nil), environment...),
		name:        name,
		args:        append([]string(nil), args...),
	})
	if strings.Contains(name, "pg_dump") {
		for index, argument := range args {
			if argument == "--file" && index+1 < len(args) {
				return os.WriteFile(args[index+1], []byte("PGDMP\x01\x0fvalidated-test-export"), 0o600)
			}
		}
	}
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestLoadConfigFailsClosed(t *testing.T) {
	values := validEnvironment()
	delete(values, "POSTGRES_STUDENT_DATABASE_URL")
	_, err := loadConfig(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "POSTGRES_STUDENT_DATABASE_URL") {
		t.Fatalf("expected missing database URL error, got %v", err)
	}

	values = validEnvironment()
	values["POSTGRES_DATABASES"] = "identity,identity"
	_, err = loadConfig(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate database rejection, got %v", err)
	}

	values = validEnvironment()
	values["DR_POSTGRES_BACKUP_HEARTBEAT_URL"] = "http://monitor.invalid"
	_, err = loadConfig(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "HTTPS URL") {
		t.Fatalf("expected HTTPS monitor validation error, got %v", err)
	}

	values = validEnvironment()
	values["DR_BACKUP_RETENTION_DAYS"] = "29"
	_, err = loadConfig(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "at least 30") {
		t.Fatalf("expected retention rejection, got %v", err)
	}
}

func TestExecuteBackupCreatesValidatedImmutableExports(t *testing.T) {
	var uploaded [][]byte
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		uploaded = append(uploaded, body)
		if !strings.HasPrefix(request.URL.Path, "/dr-bucket/auraedu/postgres/2026/07/20/") {
			t.Fatalf("unexpected object path %s", request.URL.Path)
		}
		for name, want := range map[string]string{
			"If-None-Match":                       "*",
			"X-Amz-Server-Side-Encryption":        "AES256",
			"X-Amz-Object-Lock-Mode":              "COMPLIANCE",
			"X-Amz-Object-Lock-Retain-Until-Date": "2026-08-24T04:35:00Z",
			"X-Amz-Meta-Auraedu-Pg-Major":         "18",
		} {
			if got := request.Header.Get(name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		if request.Header.Get("X-Amz-Meta-Auraedu-Sha256") == "" {
			t.Error("missing checksum metadata")
		}
		return testResponse(http.StatusOK), nil
	})}
	endpoint, _ := url.Parse("https://objects.example")
	runner := &fakeRunner{}
	now := time.Date(2026, 7, 20, 4, 35, 0, 0, time.UTC)
	cfg := config{
		Databases: []database{
			{Name: "identity", URL: "postgres://identity:secret@identity-db/identity"},
			{Name: "student", URL: "postgres://student:secret@student-db/student"},
		},
		PGDump: "/usr/local/bin/pg_dump", PGRestore: "/usr/local/bin/pg_restore",
		S3Endpoint: endpoint, S3Region: "eu-central-1", S3Bucket: "dr-bucket", S3Prefix: "auraedu",
		S3AccessKey: "access", S3SecretKey: "secret", RetentionDays: 35,
	}
	result, err := executeBackup(context.Background(), cfg, runner, client, now, t.TempDir())
	if err != nil {
		t.Fatalf("execute backup: %v", err)
	}
	if len(result.Objects) != 2 || len(uploaded) != 2 || result.TotalBytes == 0 {
		t.Fatalf("unexpected result %+v uploads=%d", result, len(uploaded))
	}
	if len(runner.calls) != 4 {
		t.Fatalf("expected dump and validation for each database, got %d calls", len(runner.calls))
	}
	for _, call := range runner.calls {
		for _, argument := range call.args {
			if strings.Contains(argument, "secret") {
				t.Fatalf("database credentials leaked into command arguments: %v", call.args)
			}
		}
	}
	dumpEnvironment := strings.Join(runner.calls[0].environment, " ")
	for _, required := range []string{"PGHOST=identity-db", "PGPORT=5432", "PGUSER=identity", "PGPASSWORD=secret", "PGDATABASE=identity"} {
		if !strings.Contains(dumpEnvironment, required) {
			t.Fatalf("pg_dump environment %q is missing %q", dumpEnvironment, required)
		}
	}
	if runner.calls[1].name != "/usr/local/bin/pg_restore" || !contains(runner.calls[1].args, "--list") {
		t.Fatalf("export catalogue was not validated: %+v", runner.calls[1])
	}
	for _, object := range result.Objects {
		if !strings.Contains(object.ObjectKey, object.Database+"-20260720T043500Z.dump") || object.SHA256 == "" {
			t.Errorf("invalid object result %+v", object)
		}
	}
}

func TestSignV4MatchesAWSPutObjectReference(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPut, "https://examplebucket.s3.amazonaws.com/test%24file.text", nil)
	request.Header.Set("Date", "Fri, 24 May 2013 00:00:00 GMT")
	request.Header.Set("X-Amz-Date", "20130524T000000Z")
	request.Header.Set("X-Amz-Storage-Class", "REDUCED_REDUNDANCY")
	payloadHash := "44ce7dd67c959e0d3524ffac1771dfbba87d2b6b4b4e99e42034a8b803f8b072"
	request.Header.Set("X-Amz-Content-Sha256", payloadHash)
	cfg := config{S3Region: "us-east-1", S3AccessKey: "unit-test-access", S3SecretKey: "unit-test-secret"}
	signV4(request, cfg, payloadHash, time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC))
	want := "AWS4-HMAC-SHA256 Credential=unit-test-access/20130524/us-east-1/s3/aws4_request,SignedHeaders=date;host;x-amz-content-sha256;x-amz-date;x-amz-storage-class,Signature=1171458f713cadceefd2d9a6911c804236f3e7e50f99757a2b10b2c6cedb2df8"
	if got := request.Header.Get("Authorization"); got != want {
		t.Fatalf("AWS SigV4 reference mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestSendStatusUsesBearerAuth(t *testing.T) {
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
}

func validEnvironment() map[string]string {
	return map[string]string{
		"POSTGRES_DATABASES":                 "identity,student",
		"POSTGRES_IDENTITY_DATABASE_URL":     "postgres://identity:secret@identity-db/identity",
		"POSTGRES_STUDENT_DATABASE_URL":      "postgresql://student:secret@student-db/student?sslmode=require",
		"DR_BACKUP_S3_ENDPOINT":              "https://s3.eu-central-1.amazonaws.com",
		"DR_BACKUP_S3_BUCKET":                "auraedu-dr",
		"DR_BACKUP_S3_ACCESS_KEY_ID":         "access",
		"DR_BACKUP_S3_SECRET_ACCESS_KEY":     "secret",
		"DR_POSTGRES_BACKUP_HEARTBEAT_URL":   "https://monitor.example/postgres-heartbeat",
		"DR_POSTGRES_BACKUP_HEARTBEAT_TOKEN": "heartbeat-token",
		"DR_POSTGRES_BACKUP_ALERT_URL":       "https://monitor.example/postgres-alert",
		"DR_POSTGRES_BACKUP_ALERT_TOKEN":     "alert-token",
	}
}

func testResponse(status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestFakeRunnerCreatesDumpInWorkspace(t *testing.T) {
	runner := &fakeRunner{}
	target := filepath.Join(t.TempDir(), "test.dump")
	if err := runner.Run(context.Background(), nil, "pg_dump", "--file", target); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(target); err != nil || info.Size() == 0 {
		t.Fatalf("fake dump missing: %v", err)
	}
}

func TestMergeEnvironmentReplacesInheritedDatabaseSettings(t *testing.T) {
	merged := mergeEnvironment(
		[]string{"PATH=/usr/bin", "PGDATABASE=", "PGAPPNAME=postgres-image"},
		[]string{"PGDATABASE=postgres://db.example/identity", "PGAPPNAME=auraedu-postgres-backup"},
	)
	if got := strings.Join(merged, "|"); got != "PATH=/usr/bin|PGDATABASE=postgres://db.example/identity|PGAPPNAME=auraedu-postgres-backup" {
		t.Fatalf("unexpected merged environment %q", got)
	}
}

func TestLibpqEnvironmentRejectsUnsupportedURLParameters(t *testing.T) {
	_, err := libpqEnvironment("postgres://user:secret@db.example/identity?options=-csearch_path%3Dpublic")
	if err == nil || !strings.Contains(err.Error(), "unsupported parameter") {
		t.Fatalf("expected unsupported connection parameter rejection, got %v", err)
	}
}
