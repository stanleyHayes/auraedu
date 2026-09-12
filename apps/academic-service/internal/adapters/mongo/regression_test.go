package mongo

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/auraedu/academic-service/internal/domain"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

func regressionStore(t *testing.T) *pmongo.Store {
	t.Helper()
	s := testkit.NewMongoReplicaSet(context.Background(), t).Store
	if err := EnsureIndexes(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestMongoTimetableConcurrentOverlap(t *testing.T) {
	ctx := context.Background()
	s := regressionStore(t)
	r := NewTimetableRepository(s)
	year := &domain.AcademicYear{ID: "year", TenantID: "tenant", Code: "year", Name: "Year"}
	if err := NewRepository(s).Create(ctx, "tenant", year); err != nil {
		t.Error(err)
		return
	}
	if err := NewSubjectRepository(s).Create(ctx, "tenant", &domain.Subject{ID: "subject", TenantID: "tenant", Name: "Math"}); err != nil {
		t.Error(err)
		return
	}
	for i := -1; i < 12; i++ {
		id := fmt.Sprintf("class-%d", i)
		if i == -1 {
			id = "class"
		}
		if err := NewClassRepository(s).Create(ctx, "tenant", &domain.Class{ID: id, TenantID: "tenant", Name: id, AcademicYearID: "year"}); err != nil {
			t.Error(err)
			return
		}
	}
	for _, sameClass := range []bool{true, false} {
		teacher := "teacher"
		term := uuid.NewString()
		if err := NewTermRepository(s).Create(ctx, "tenant", &domain.Term{ID: term, TenantID: "tenant", AcademicYearID: "year", Name: term}); err != nil {
			t.Error(err)
			return
		}
		var wins atomic.Int32
		var wg sync.WaitGroup
		errs := make(chan error, 12)
		for i := range 12 {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				class := fmt.Sprintf("class-%d", i)
				if sameClass {
					class = "class"
				}
				entry, err := domain.NewTimetableEntry("tenant", class, term, "subject", &teacher, 1, "08:00", "09:00", nil)
				if err != nil {
					t.Error(err)
					return
				}
				err = r.Create(ctx, "tenant", entry)
				if err == nil {
					wins.Add(1)
				} else if !errors.Is(err, domain.ErrConflict) {
					errs <- err
				}
			}(i)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Error(err)
			return
		}
		if wins.Load() != 1 {
			t.Fatalf("overlap winners %d", wins.Load())
		}
	}
}
