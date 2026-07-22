package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/campaign-service/internal/adapters/memory"
	"github.com/auraedu/campaign-service/internal/application"
	"github.com/auraedu/campaign-service/internal/domain"
	"github.com/auraedu/platform/auth"
)

func actor(id string, perms ...string) auth.Actor {
	return auth.Actor{UserID: id, TenantID: "school-one", Permissions: perms}
}
func TestCampaignFourEyesLifecycle(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	svc := application.NewService(memory.New(), application.WithClock(func() time.Time { return now }))
	owner := actor("owner", application.PermCreate, application.PermRead, application.PermUpdate)
	campaign, err := svc.Create(context.Background(), owner, application.CreateInput{Name: "Open day 2026", Objective: "Increase qualified applications", Channel: "instagram", AudienceDefinition: "Prospective students in Greater Accra", Budget: 5000, Currency: "GHS", StartAt: now.Add(time.Hour), EndAt: now.Add(30 * 24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if campaign.TrackingURLParameters == "" {
		t.Fatal("tracking parameters missing")
	}
	campaign, err = svc.Submit(context.Background(), owner, campaign.ID)
	if err != nil || campaign.Status != domain.StatusPending {
		t.Fatalf("submit=%+v err=%v", campaign, err)
	}
	if _, err = svc.Approve(context.Background(), actor("owner", application.PermApprove, application.PermBudgetApprove), campaign.ID, "approved"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("self approval=%v", err)
	}
	reviewer := actor("reviewer", application.PermApprove, application.PermBudgetApprove)
	campaign, err = svc.Approve(context.Background(), reviewer, campaign.ID, "Audience and budget verified")
	if err != nil || campaign.Status != domain.StatusApproved {
		t.Fatalf("approve=%+v err=%v", campaign, err)
	}
	campaign, err = svc.Publish(context.Background(), actor("publisher", application.PermPublish), campaign.ID)
	if err != nil || campaign.Status != domain.StatusScheduled {
		t.Fatalf("publish=%+v err=%v", campaign, err)
	}
	campaign, err = svc.Pause(context.Background(), actor("publisher", application.PermPublish), campaign.ID)
	if err != nil || campaign.Status != domain.StatusPaused {
		t.Fatalf("pause=%+v err=%v", campaign, err)
	}
}
func TestCampaignBudgetApprovalPermission(t *testing.T) {
	now := time.Now().UTC()
	svc := application.NewService(memory.New(), application.WithClock(func() time.Time { return now }))
	owner := actor("owner", application.PermCreate, application.PermUpdate)
	campaign, err := svc.Create(context.Background(), owner, application.CreateInput{Name: "Email nurture", Objective: "Increase applications", Channel: "email", AudienceDefinition: "Consented enquiries", Budget: 1, Currency: "GHS", StartAt: now.Add(time.Hour), EndAt: now.Add(48 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	campaign, err = svc.Submit(context.Background(), owner, campaign.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Approve(context.Background(), actor("reviewer", application.PermApprove), campaign.ID, "Reviewed")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("budget approved without permission: %v", err)
	}
}
