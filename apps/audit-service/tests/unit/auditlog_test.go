package unit

import (
	"testing"
	"time"

	"github.com/auraedu/audit-service/internal/domain"
	"github.com/google/uuid"
)

func TestAuditLogBuilder_Success(t *testing.T) {
	tenantID := "school-a"
	log, err := domain.NewAuditLogBuilder().
		TenantID(tenantID).
		EventID("evt-1").
		EventType("student.created.v1").
		SourceService("student-service").
		Timestamp(time.Now().UTC()).
		ReceivedAt(time.Now().UTC()).
		Action("student.created.v1").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if log.ID == uuid.Nil {
		t.Fatal("expected generated id")
	}
	if log.TenantID != tenantID {
		t.Fatalf("tenant mismatch: got %s, want %s", log.TenantID, tenantID)
	}
}

func TestAuditLogBuilder_MissingTenant(t *testing.T) {
	_, err := domain.NewAuditLogBuilder().
		EventID("evt-1").
		EventType("student.created.v1").
		SourceService("student-service").
		Timestamp(time.Now().UTC()).
		ReceivedAt(time.Now().UTC()).
		Action("student.created.v1").
		Build()
	if err == nil {
		t.Fatal("expected validation error for missing tenant")
	}
}

func TestAuditLogBuilder_MissingEventID(t *testing.T) {
	tenantID := "school-a"
	_, err := domain.NewAuditLogBuilder().
		TenantID(tenantID).
		EventType("student.created.v1").
		SourceService("student-service").
		Timestamp(time.Now().UTC()).
		ReceivedAt(time.Now().UTC()).
		Action("student.created.v1").
		Build()
	if err == nil {
		t.Fatal("expected validation error for missing event id")
	}
}

func TestAuditLogBuilder_GeneratesID(t *testing.T) {
	tenantID := "school-a"
	log, err := domain.NewAuditLogBuilder().
		TenantID(tenantID).
		EventID("evt-1").
		EventType("student.created.v1").
		SourceService("student-service").
		Timestamp(time.Now().UTC()).
		ReceivedAt(time.Now().UTC()).
		Action("student.created.v1").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if log.ID == uuid.Nil {
		t.Fatal("expected generated id")
	}
}
