// Package events adapts the platform eventbus to the assessment-service EventPublisher port.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/assessment-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
)

// Publisher adapts the platform eventbus to the assessment service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// PublishAssessment emits a CloudEvent for the given assessment domain event.
func (p *Publisher) PublishAssessment(ctx context.Context, eventType string, a *domain.Assessment, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"assessment_id":    a.ID,
		"academic_year_id": a.AcademicYearID,
		"subject_id":       a.SubjectID,
		"type":             a.Type,
		"title":            a.Title,
		"max_score":        a.MaxScore,
		"status":           a.Status,
	}
	if a.Description != nil {
		data["description"] = *a.Description
	}
	if a.DueDate != nil {
		data["due_date"] = a.DueDate.Format(time.RFC3339)
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "assessment-service", "", a.TenantID, data)
	if err != nil {
		return fmt.Errorf("assessment: build event: %w", err)
	}
	event.Subject = a.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishAssignment emits a CloudEvent for the given assignment domain event.
// The payload conforms to contracts/events/assignment.published.v1.json.
func (p *Publisher) PublishAssignment(ctx context.Context, eventType string, a *domain.Assessment, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"assignment_id": a.ID,
		"subject_id":    a.SubjectID,
		"title":         a.Title,
	}
	if len(a.ClassIDs) > 0 {
		data["class_ids"] = a.ClassIDs
	}
	if a.DueDate != nil {
		data["due_date"] = a.DueDate.UTC().Format(time.RFC3339)
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "assessment-service", "", a.TenantID, data)
	if err != nil {
		return fmt.Errorf("assessment: build assignment event: %w", err)
	}
	event.Subject = a.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishScore emits a CloudEvent for the given score domain event.
func (p *Publisher) PublishScore(ctx context.Context, eventType string, s *domain.Score, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"score_id":      s.ID,
		"assessment_id": s.AssessmentID,
		"student_id":    s.StudentID,
		"score":         s.Score,
		"recorded_by":   s.RecordedBy,
	}
	if s.Notes != nil {
		data["notes"] = *s.Notes
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "assessment-service", "", s.TenantID, data)
	if err != nil {
		return fmt.Errorf("assessment: build score event: %w", err)
	}
	event.Subject = s.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
