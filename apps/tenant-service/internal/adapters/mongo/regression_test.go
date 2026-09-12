package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func regressionStore(t *testing.T) *pmongo.Store {
	t.Helper()
	s := testkit.NewMongoReplicaSet(context.Background(), t).Store
	for range 16 {
		if err := EnsureIndexes(context.Background(), s); err != nil {
			t.Fatal(err)
		}
	}
	return s
}
func validation(t *testing.T, s *pmongo.Store, name string, rule bson.M) {
	t.Helper()
	if err := s.Database().RunCommand(context.Background(), bson.D{{Key: "collMod", Value: name}, {Key: "validator", Value: rule}, {Key: "validationLevel", Value: "strict"}}).Err(); err != nil {
		t.Fatal(err)
	}
}
func TestMongoOnboardingAtomicApprovalAndRevocation(t *testing.T) {
	ctx := context.Background()
	s := regressionStore(t)
	r := NewRepository(s)
	validation(t, s, FeatureCollection, bson.M{"_reject_test": bson.M{"$exists": true}})
	broken := domain.Tenant{Code: "broken", Name: "Broken", Plan: "starter", Status: "active"}
	if err := r.CreateTenant(ctx, broken); err == nil {
		t.Fatal("seed fault did not fail")
	}
	if _, err := r.GetTenant(ctx, "broken"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("partial tenant %v", err)
	}
	validation(t, s, FeatureCollection, bson.M{})
	request := &domain.OnboardingRequest{ID: uuid.NewString(), SchoolName: "School", AdministratorName: "Admin", Email: "admin@example.test", CountryCode: "GH", Plan: "starter", Status: domain.OnboardingPending, SubmittedAt: time.Now()}
	if _, _, err := r.SubmitOnboarding(ctx, request, "key", "payload", "email"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var winners atomic.Int32
	errs := make(chan error, 8)
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tenant := domain.Tenant{Code: fmt.Sprintf("school-%d", i), Name: "School", Plan: "starter", Status: "onboarding"}
			_, err := r.ApproveOnboarding(ctx, request.ID, tenant, "admin")
			if err == nil {
				winners.Add(1)
			} else if !errors.Is(err, domain.ErrConflict) {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if winners.Load() != 1 {
		t.Fatalf("approval winners %d", winners.Load())
	}
	tenants, err := r.ListTenants(ctx)
	if err != nil || len(tenants) != 1 {
		t.Fatalf("orphan tenants %+v %v", tenants, err)
	}
	code := tenants[0].Code
	settings, err := r.Settings(ctx, code)
	if err != nil || settings.PrimaryContactEmail != request.Email {
		t.Fatalf("contact missing %+v %v", settings, err)
	}
	events, err := r.ClaimPending(ctx, 100)
	if err != nil || len(events) != 2 {
		t.Fatalf("approval outbox %d %v", len(events), err)
	}
	approved := false
	for _, e := range events {
		if e.EventType == "tenant.onboarding_approved.v1" {
			var payload map[string]any
			if err = json.Unmarshal(e.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			approved = payload["request_id"] == request.ID && payload["tenant_code"] == code
		}
		if err = r.MarkPublished(ctx, e.ID); err != nil {
			t.Fatal(err)
		}
	}
	if !approved {
		t.Fatal("invite trigger missing")
	}

	registration := domain.CustomDomain{TenantCode: code, Hostname: "school.example.org", Status: domain.DomainPending, TXTRecordName: "_auraedu.school.example.org"}
	if _, err = r.RequestCustomDomain(ctx, registration, "hash"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.MarkCustomDomainVerified(ctx, code, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ActivateCustomDomain(ctx, code, "provider", time.Now()); err != nil {
		t.Fatal(err)
	}
	routed, err := r.GetTenant(ctx, code)
	if err != nil || routed.Domain != registration.Hostname {
		t.Fatalf("domain routing missing %+v %v", routed, err)
	}
	if _, err = r.DeactivateCustomDomain(ctx, code, "provider", time.Now()); err != nil {
		t.Fatal(err)
	}
	routed, err = r.GetTenant(ctx, code)
	if err != nil || routed.Domain != "" {
		t.Fatalf("domain revocation missing %+v %v", routed, err)
	}
	if err = r.DeleteTenant(ctx, code); err != nil {
		t.Fatal(err)
	}
	if _, err = r.GetTenant(ctx, code); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted tenant visible %v", err)
	}
	if _, err = r.Features(ctx, code); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted flags visible %v", err)
	}
	tenants, err = r.ListTenants(ctx)
	if err != nil || len(tenants) != 0 {
		t.Fatalf("deleted tenant listed %+v %v", tenants, err)
	}
	events, err = r.ClaimPending(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if err = r.MarkPublished(ctx, e.ID); err != nil {
			t.Fatal(err)
		}
	}
	replacement := domain.Tenant{Code: code, Name: "New School", Plan: "basic", Status: "active"}
	if err = r.CreateTenant(ctx, replacement); err != nil {
		t.Fatal(err)
	}
	flags, err := r.Features(ctx, code)
	if err != nil || len(flags) == 0 {
		t.Fatalf("reused tenant missing feature defaults: %v %v", flags, err)
	}
	if _, _, err = r.GetCustomDomain(ctx, code); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("old domain revived: %v", err)
	}
}
