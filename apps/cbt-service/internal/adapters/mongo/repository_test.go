package mongo

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/auraedu/cbt-service/internal/domain"
	"github.com/auraedu/cbt-service/internal/ports"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
)

func TestMongoLifecycleAndIsolation(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	r := NewRepository(s)
	if err := r.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	q := &domain.QuestionBank{ID: "q1", TenantID: "a", AcademicYearID: "year", SubjectID: "math", CorrectAnswer: "secret", Status: "draft", CreatedAt: time.Now()}
	es := []ports.LifecycleEvent{{EventType: "cbt.question_created.v1", Payload: map[string]any{"question_id": q.ID}}}
	if err := r.CommitCBTLifecycle(ctx, "a", ports.LifecycleMutation{Kind: ports.CBTMutationQuestionCreate, Question: q}, es); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetQuestionByID(ctx, "a", q.ID)
	if err != nil || got.CorrectAnswer != "secret" {
		t.Fatalf("roundtrip: %+v %v", got, err)
	}
	if _, err = r.GetQuestionByID(ctx, "b", q.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross tenant read: %v", err)
	}
	if err = r.DeleteQuestion(ctx, "b", q.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross tenant delete: %v", err)
	}
	if _, err = r.GetQuestionByID(ctx, "", q.ID); !errors.Is(err, pmongo.ErrTenantRequired) {
		t.Fatalf("unscoped read: %v", err)
	}
	var count atomic.Int64
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			items, err := r.ClaimPendingCBTEvents(ctx, 25)
			if err != nil {
				t.Error(err)
				return
			}
			count.Add(int64(len(items)))
		})
	}
	wg.Wait()
	if count.Load() != 1 {
		t.Fatalf("outbox claimed %d times", count.Load())
	}
	if err = r.CommitCBTLifecycle(ctx, "a", ports.LifecycleMutation{Kind: ports.CBTMutationQuestionDelete, Question: q}, es); err != nil {
		t.Fatal(err)
	}
	if _, err = r.GetQuestionByID(ctx, "a", q.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	items, err := r.ClaimPendingCBTEvents(ctx, 25)
	if err != nil || len(items) != 1 {
		t.Fatalf("delete event lost: %v %v", items, err)
	}
	// A malformed event cannot create its aggregate.
	q.ID = "invalid"
	err = r.CommitCBTLifecycle(ctx, "a", ports.LifecycleMutation{Kind: ports.CBTMutationQuestionCreate, Question: q}, []ports.LifecycleEvent{{EventType: "", Payload: map[string]any{}}})
	if err == nil {
		t.Fatal("invalid event accepted")
	}
	if _, err = r.GetQuestionByID(ctx, "a", q.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("partial lifecycle commit", err)
	}
	// Database uniqueness, not an application read-before-write, elects one attempt.
	var created atomic.Int64
	for n := range 12 {
		wg.Go(func() {
			v := &domain.Submission{ID: fmt.Sprint(n), TenantID: "a", ExamSessionID: "exam", StudentID: "student"}
			err := r.CreateSubmission(ctx, "a", v)
			if err == nil {
				created.Add(1)
			} else if !errors.Is(err, domain.ErrConflict) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if created.Load() != 1 {
		t.Fatalf("created %d submissions", created.Load())
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
