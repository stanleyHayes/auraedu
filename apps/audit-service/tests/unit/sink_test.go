package unit

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/auraedu/audit-service/internal/adapters/memory"
	"github.com/auraedu/audit-service/internal/application"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
)

func TestSink_Process(t *testing.T) {
	repo := memory.NewRepository()
	sink := application.NewSink(repo)

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{
		TenantID: tenantID.String(),
		ActorID:  "user-123",
	})

	event := tenancy.CloudEvent{
		SpecVersion: "1.0",
		Type:        "student.created.v1",
		Source:      "student-service",
		ID:          "evt-1",
		Time:        "2024-01-01T00:00:00Z",
		TenantID:    tenantID.String(),
		Data:        json.RawMessage(`{"id":"stu-1"}`),
		Subject:     "stu-1",
	}

	if err := sink.Process(ctx, event); err != nil {
		t.Fatalf("process: %v", err)
	}

	logs, _, err := repo.List(ctx, tenantID.String(), 10, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	log := logs[0]
	if log.ActorID != "user-123" {
		t.Fatalf("actor id mismatch: got %s, want user-123", log.ActorID)
	}
	if log.Action != "student.created.v1" {
		t.Fatalf("action mismatch: got %s", log.Action)
	}
	if log.ResourceType != "student" {
		t.Fatalf("resource type mismatch: got %s", log.ResourceType)
	}
	if log.ResourceID != "stu-1" {
		t.Fatalf("resource id mismatch: got %s", log.ResourceID)
	}
	if log.SourceService != "student-service" {
		t.Fatalf("source service mismatch: got %s", log.SourceService)
	}
}

func TestSink_Process_MissingTenant(t *testing.T) {
	repo := memory.NewRepository()
	sink := application.NewSink(repo)

	event := tenancy.CloudEvent{
		SpecVersion: "1.0",
		Type:        "student.created.v1",
		Source:      "student-service",
		ID:          "evt-1",
		Time:        "2024-01-01T00:00:00Z",
	}

	if err := sink.Process(context.Background(), event); err == nil {
		t.Fatal("expected error for missing tenant")
	}
}

func TestSink_Process_ActorFromPayload(t *testing.T) {
	repo := memory.NewRepository()
	sink := application.NewSink(repo)

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{
		TenantID: tenantID.String(),
	})

	event := tenancy.CloudEvent{
		SpecVersion: "1.0",
		Type:        "student.created.v1",
		Source:      "student-service",
		ID:          "evt-1",
		Time:        "2024-01-01T00:00:00Z",
		TenantID:    tenantID.String(),
		Data:        json.RawMessage(`{"actorid":"user-456"}`),
		Subject:     "stu-2",
	}

	if err := sink.Process(ctx, event); err != nil {
		t.Fatalf("process: %v", err)
	}

	logs, _, err := repo.List(ctx, tenantID.String(), 10, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].ActorID != "user-456" {
		t.Fatalf("actor id mismatch: got %s, want user-456", logs[0].ActorID)
	}
}
