package adapters_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/file-service/internal/ports"
	"github.com/stretchr/testify/require"

	mongoadapter "github.com/auraedu/file-service/internal/adapters/mongo"
	"github.com/auraedu/platform/testkit"
)

func TestMongoFileUsageAndUniquePath(t *testing.T) {
	ctx := context.Background()
	db := testkit.NewMongo(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, db.Store); err != nil {
		t.Fatal(err)
	}
	r := mongoadapter.NewRepository(db.Store)
	a := newFile("a")
	if err := r.Create(ctx, "a", a); err != nil {
		t.Fatal(err)
	}
	b := newFile("a")
	b.StoragePath = a.StoragePath
	if err := r.Create(ctx, "a", b); err == nil {
		t.Fatal("duplicate storage path accepted")
	}
	if err := r.RecordStorage(ctx, "a", 1); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := r.RecordStorage(ctx, "a", 10); err != nil {
				t.Error(err)
			}
			if err := r.RecordDelivery(ctx, "a", 20); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	usage, err := r.GetUsage(ctx, "a", 100)
	if err != nil || len(usage) != 1 || usage[0].BytesStored != 81 || usage[0].BytesDelivered != 160 {
		t.Fatalf("usage %+v %v", usage, err)
	}
	other, err := r.GetUsage(ctx, "b", 100)
	if err != nil || len(other) != 0 {
		t.Fatal("usage tenant leak", err)
	}
}

func TestMongoFileDeletionPreservesOutboxAndCleanup(t *testing.T) {
	ctx := context.Background()
	db := testkit.NewMongo(ctx, t)
	r := mongoadapter.NewRepository(db.Store)
	a := newFile("a")
	payload := map[string]any{"file_id": a.ID}
	require.NoError(t, r.CommitFileLifecycle(ctx, "a", a, ports.FileMutationCreate, "file.uploaded.v1", payload))
	require.NoError(t, r.CommitFileLifecycle(ctx, "a", a, ports.FileMutationDelete, "file.deleted.v1", payload))
	_, err := r.GetByID(ctx, "a", a.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	files, _, err := r.List(ctx, "a", 100, "")
	require.NoError(t, err)
	require.Empty(t, files)
	require.ErrorIs(t, r.Update(ctx, "a", a), domain.ErrNotFound)
	require.ErrorIs(t, r.CommitFileLifecycle(ctx, "a", a, ports.FileMutationUpdate, "file.updated.v1", payload), domain.ErrNotFound)
	events, err := r.ClaimPendingFileEvents(ctx, 100)
	require.NoError(t, err)
	require.Len(t, events, 2)
	for _, event := range events {
		if event.EventType == "file.deleted.v1" {
			require.Equal(t, a.StoragePath, event.CleanupPath)
		}
		require.NoError(t, r.MarkFileEventPublished(ctx, event.ID))
	}
	b := newFile("a")
	require.NoError(t, r.CommitFileLifecycle(ctx, "a", b, ports.FileMutationCreate, "file.uploaded.v1", payload))
	require.NoError(t, r.Delete(ctx, "a", b.ID))
	require.True(t, errors.Is(r.Delete(ctx, "a", b.ID), domain.ErrNotFound))
	events, err = r.ClaimPendingFileEvents(ctx, 100)
	require.NoError(t, err)
	require.Len(t, events, 1)
}
