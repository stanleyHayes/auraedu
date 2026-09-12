package adapters_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/auraedu/platform/testkit"
	mongoadapter "github.com/auraedu/staff-service/internal/adapters/mongo"
	"github.com/auraedu/staff-service/internal/domain"
	"github.com/auraedu/staff-service/internal/ports"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMongoStaffReferenceAndUniqueRegressions(t *testing.T) {
	ctx := context.Background()
	db := testkit.NewMongoReplicaSet(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, db.Store); err != nil {
		t.Fatal(err)
	}
	r := mongoadapter.NewRepository(db.Store)
	s := newStaff("a", "CODE")
	if err := r.Create(ctx, "a", s); err != nil {
		t.Fatal(err)
	}
	duplicate := newStaff("a", "CODE")
	if err := r.Create(ctx, "a", duplicate); err == nil {
		t.Fatal("duplicate staff code accepted")
	}
	class := uuid.NewString()
	var wins atomic.Int64
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			a, err := domain.NewAssignment("a", s.ID, class, nil, nil)
			if err != nil {
				t.Error(err)
				return
			}
			if err = r.CreateAssignment(ctx, "a", a, ports.AssignmentEventData(a)); err == nil {
				wins.Add(1)
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("duplicate assignments accepted %d", wins.Load())
	}
	foreign, err := domain.NewAssignment("b", s.ID, uuid.NewString(), nil, nil)
	require.NoError(t, err)
	if err := r.CreateAssignment(ctx, "b", foreign, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("foreign staff assignment", err)
	}
	if err := r.Delete(ctx, "a", s.ID); err != nil {
		t.Fatal(err)
	}
	assignments, _, err := r.ListAssignments(ctx, "a", s.ID, 100, "")
	if err != nil || len(assignments) != 0 {
		t.Fatal("deleted staff assignment still visible", err)
	}
	classes, err := r.ListAssignmentClassIDs(ctx, "a", s.ID)
	if err != nil || len(classes) != 0 {
		t.Fatal("deleted staff retains class authority", err)
	}
}
