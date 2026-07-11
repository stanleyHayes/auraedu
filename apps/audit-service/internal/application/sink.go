// Package application implements the audit sink use case.
package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/auraedu/audit-service/internal/domain"
	"github.com/auraedu/audit-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
)

// Sink is the audit use case: it turns a CloudEvent into an immutable AuditLog
// and persists it. Tenant scoping and actor attribution happen here.
type Sink struct {
	repo ports.Repository
}

// NewSink creates a new audit sink backed by the given repository.
func NewSink(repo ports.Repository) *Sink {
	return &Sink{repo: repo}
}

// Process extracts tenant and resource metadata from a CloudEvent, builds an
// AuditLog, and persists it. The context must carry the tenant context; ActorID
// is taken from the context first, then from payload fallbacks.
func (s *Sink) Process(ctx context.Context, event tenancy.CloudEvent) error {
	tenantID, err := extractTenantID(event)
	if err != nil {
		return err
	}

	actorID := ""
	if tc, ok := tenancy.FromContext(ctx); ok {
		if tc.ActorID != "" {
			actorID = tc.ActorID
		}
	}
	if actorID == "" {
		actorID = extractActorIDFromPayload(event.Data)
	}

	ts, err := parseEventTime(event.Time)
	if err != nil {
		return fmt.Errorf("audit sink: invalid event time: %w", err)
	}
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	resourceType, resourceID := extractResource(event)

	log, err := domain.NewAuditLogBuilder().
		TenantID(tenantID).
		EventID(event.ID).
		EventType(event.Type).
		SourceService(event.Source).
		Timestamp(ts).
		ReceivedAt(time.Now().UTC()).
		Payload(event.Data).
		ActorID(actorID).
		Action(event.Type).
		ResourceType(resourceType).
		ResourceID(resourceID).
		Build()
	if err != nil {
		return fmt.Errorf("audit sink: build audit log: %w", err)
	}

	if err := s.repo.Insert(ctx, log); err != nil {
		return fmt.Errorf("audit sink: insert: %w", err)
	}
	return nil
}

func extractTenantID(event tenancy.CloudEvent) (uuid.UUID, error) {
	if event.TenantID != "" {
		id, err := uuid.Parse(event.TenantID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("audit sink: invalid tenant_id %q: %w", event.TenantID, err)
		}
		return id, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(event.Data, &payload); err == nil {
		if v, ok := payload["tenant_id"].(string); ok && v != "" {
			id, err := uuid.Parse(v)
			if err != nil {
				return uuid.Nil, fmt.Errorf("audit sink: invalid payload tenant_id %q: %w", v, err)
			}
			return id, nil
		}
	}
	return uuid.Nil, tenancy.ErrMissingEventTenant
}

func extractActorIDFromPayload(data json.RawMessage) string {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	if v, ok := payload["actor_id"].(string); ok {
		return v
	}
	if v, ok := payload["actorid"].(string); ok {
		return v
	}
	return ""
}

func extractResource(event tenancy.CloudEvent) (resourceType, resourceID string) {
	resourceID = event.Subject
	parts := strings.SplitN(event.Type, ".", 2)
	if len(parts) > 0 && parts[0] != "" {
		resourceType = parts[0]
	}
	return resourceType, resourceID
}

func parseEventTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}
