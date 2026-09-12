package mongo

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/auraedu/admissions-service/internal/domain"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
)

func TestMongoCatalogueAdmissionAtomicity(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	r := NewRepository(s)
	if err := r.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	p := domain.Programme{ID: "programme", TenantID: "a", Code: "P", Slug: "p", Name: "Programme", Status: domain.ProgrammePublished, Version: 1}
	if err := r.CreateProgramme(ctx, p); err != nil {
		t.Fatal(err)
	}
	i := domain.Intake{ID: "intake", TenantID: "a", ProgrammeID: p.ID, Name: "Intake", Status: domain.IntakeOpen, Version: 1, ApplicationOpensAt: now.Add(-time.Hour), ApplicationClosesAt: now.Add(time.Hour)}
	if err := r.CreateIntake(ctx, i); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetProgramme(ctx, "b", p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("tenant isolation", err)
	}
	var created atomic.Int64
	var wg sync.WaitGroup
	for n := range 8 {
		wg.Go(func() {
			a := domain.Application{ID: fmt.Sprint(n), TenantID: "a", ApplicantUserID: "student", ProgrammeID: p.ID, IntakeID: i.ID, Status: domain.StatusDraft, CreatedAt: now}
			err := r.CreateForAvailableIntake(ctx, a, now, "application.started.v1", map[string]any{"application_id": a.ID})
			if err == nil {
				created.Add(1)
			} else if !errors.Is(err, domain.ErrConflict) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if created.Load() != 1 {
		t.Fatalf("concurrent admission created %d", created.Load())
	}
	items, err := r.ClaimPending(ctx, 100)
	if err != nil || len(items) != 1 {
		t.Fatalf("outbox %v %v", items, err)
	}
	as, err := r.List(ctx, "a", "student", "", 100)
	if err != nil || len(as) != 1 || as[0].ProgrammeName != p.Name {
		t.Fatalf("snapshot %+v %v", as, err)
	}
	i.Status = domain.IntakeClosed
	i.Version = 2
	if err = r.UpdateIntake(ctx, i, 1); err != nil {
		t.Fatal(err)
	}
	if err = r.UpdateIntake(ctx, i, 1); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale version accepted %v", err)
	}
	a := domain.Application{ID: "closed", TenantID: "a", ApplicantUserID: "next", ProgrammeID: p.ID, IntakeID: i.ID}
	if err = r.CreateForAvailableIntake(ctx, a, now, "application.started.v1", nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("closed intake admitted %v", err)
	}
	if _, err = r.Get(ctx, "a", a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("partial rejected application", err)
	}
	i.Status = domain.IntakeOpen
	i.Version = 3
	if err = r.UpdateIntake(ctx, i, 2); err != nil {
		t.Fatal(err)
	}
	if err = r.CreateForAvailableIntake(ctx, a, now, "application.started.v1", map[string]any{"invalid": make(chan int)}); err == nil {
		t.Fatal("invalid event accepted")
	}
	if _, err = r.Get(ctx, "a", a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("partial invalid-event application", err)
	}
}
func testStore(t *testing.T) *pmongo.Store {
	t.Helper()
	ctx := context.Background()
	c, err := tcmongo.Run(ctx, "mongo:8.0", tcmongo.WithReplicaSet("rs0"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(c) })
	uri, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s, err := pmongo.Open(ctx, pmongo.Config{URI: uri + "&directConnection=true", Database: "adapter_test", MaxPoolSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { require.NoError(t, s.Close(context.Background())) })
	return s
}
