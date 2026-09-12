package mongo

import (
	"context"
	"errors"
	"testing"
	"time"

	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/website-service/internal/domain"
	"github.com/auraedu/website-service/internal/ports"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func regressionStore(t *testing.T) *pmongo.Store {
	t.Helper()
	s := testkit.NewMongoReplicaSet(context.Background(), t).Store
	if err := EnsureIndexes(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	return s
}
func validation(t *testing.T, s *pmongo.Store, name string, rule bson.M) {
	t.Helper()
	if err := s.Database().RunCommand(context.Background(), bson.D{{Key: "collMod", Value: name}, {Key: "validator", Value: rule}, {Key: "validationLevel", Value: "strict"}}).Err(); err != nil {
		t.Fatal(err)
	}
}
func TestMongoWebsiteProvisionRollbackAndCascade(t *testing.T) {
	ctx := context.Background()
	s := regressionStore(t)
	r := NewRepository(s)
	page := &domain.Page{ID: uuid.NewString(), TenantID: "tenant", Slug: "home", Title: "Home", Status: "draft", Layout: "default", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	section := &domain.Section{ID: uuid.NewString(), TenantID: "tenant", PageID: page.ID, Type: "hero", Content: domain.Content{}, Status: "draft", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	validation(t, s, SectionsCollection, bson.M{"_reject_test": bson.M{"$exists": true}})
	if err := r.ProvisionDefaultWebsite(ctx, "tenant", page, section, []ports.LifecycleEvent{{EventType: "website.page_created.v1", Payload: map[string]any{}}}); err == nil {
		t.Fatal("fault did not fail")
	}
	if _, err := r.GetPageByID(ctx, "tenant", page.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("partial page %v", err)
	}
	validation(t, s, SectionsCollection, bson.M{})
	if err := r.ProvisionDefaultWebsite(ctx, "tenant", page, section, []ports.LifecycleEvent{{EventType: "website.page_created.v1", Payload: map[string]any{}}}); err != nil {
		t.Fatal(err)
	}
	if err := r.CommitWebsiteLifecycle(ctx, "tenant", ports.WebsiteMutationPageDelete, page, nil, []ports.LifecycleEvent{{EventType: "website.page_deleted.v1", Payload: map[string]any{}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetSectionByID(ctx, "tenant", section.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cascade missing %v", err)
	}
	events, err := r.ClaimPendingWebsiteEvents(ctx, 100)
	if err != nil || len(events) != 2 {
		t.Fatalf("cascade erased outbox %d %v", len(events), err)
	}
}
