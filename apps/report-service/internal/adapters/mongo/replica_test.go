package mongo

import (
	"context"
	"errors"

	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"

	"testing"
)

func replicaStore(t *testing.T) *pmongo.Store {
	t.Helper()
	s := testkit.NewMongoReplicaSet(context.Background(), t).Store
	if err := EnsureIndexes(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMongoReportTemplateReferenceRace(t *testing.T) {
	ctx := context.Background()
	s := replicaStore(t)
	r := NewRepository(s)
	for range 12 {
		template, err := domain.NewReportTemplate("tenant-a", "Annual", "year", "<h1>Report</h1>")
		if err != nil {
			t.Fatal(err)
		}
		if err := r.CreateReportTemplate(ctx, "tenant-a", template); err != nil {
			t.Fatal(err)
		}
		card, err := domain.NewReportCard("tenant-a", "student", "year", template.ID)
		if err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		created := make(chan error, 1)
		deleted := make(chan error, 1)
		go func() {
			<-start
			created <- r.CommitReportCardLifecycle(ctx, "tenant-a", card, ports.ReportMutationCreate, "report.created.v1", map[string]any{"report_card_id": card.ID})
		}()
		go func() {
			<-start
			deleted <- r.CommitReportTemplateLifecycle(ctx, "tenant-a", template, ports.ReportMutationDelete, "report_template.deleted.v1", map[string]any{})
		}()
		close(start)
		createErr, deleteErr := <-created, <-deleted
		if createErr == nil {
			if !errors.Is(deleteErr, domain.ErrConflict) {
				t.Fatalf("delete did not restrict child: %v", deleteErr)
			}
			if _, err := r.GetReportTemplateByID(ctx, "tenant-a", template.ID); err != nil {
				t.Fatalf("parent rollback %v", err)
			}
			continue
		}
		if deleteErr != nil || !errors.Is(createErr, domain.ErrNotFound) {
			t.Fatalf("race outcomes create=%v delete=%v", createErr, deleteErr)
		}
		if _, err := r.GetReportCardByID(ctx, "tenant-a", card.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("orphan created %v", err)
		}
	}
}
