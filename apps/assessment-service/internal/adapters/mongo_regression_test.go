package adapters_test

import (
	"context"
	"errors"
	"testing"

	mongoadapter "github.com/auraedu/assessment-service/internal/adapters/mongo"
	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/assessment-service/internal/ports"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

func TestMongoAssessmentParentRollback(t *testing.T) {
	ctx := context.Background()
	db := testkit.NewMongoReplicaSet(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, db.Store); err != nil {
		t.Fatal(err)
	}
	r := mongoadapter.NewRepository(db.Store)
	a := newAssessment("a", uuid.NewString(), uuid.NewString())
	if err := r.CreateAssessment(ctx, "a", a); err != nil {
		t.Fatal(err)
	}
	foreign := newScore("b", a.ID, uuid.NewString())
	if err := r.CreateScore(ctx, "b", foreign); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("foreign assessment accepted", err)
	}
	score := newScore("a", a.ID, uuid.NewString())
	err := r.CommitAssessmentLifecycle(ctx, "a", ports.LifecycleMutation{Kind: ports.AssessmentMutationScoreCreate, Score: score}, []ports.LifecycleEvent{{EventType: ""}})
	if err == nil {
		t.Fatal("invalid event accepted")
	}
	if _, err = r.GetScoreByID(ctx, "a", a.ID, score.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("partial score commit", err)
	}
	if err = r.DeleteAssessment(ctx, "a", a.ID); err != nil {
		t.Fatal(err)
	}
	if err = r.CreateScore(ctx, "a", score); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("deleted assessment accepted score", err)
	}
}
