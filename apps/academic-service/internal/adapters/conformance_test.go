// Package adapters_test runs one contract against every academic repository driver.
//
// Swapping persistence is only safe if both adapters behave identically, so the
// assertions live here once and each driver runs them.
package adapters_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mongoadapter "github.com/auraedu/academic-service/internal/adapters/mongo"
	pgadapter "github.com/auraedu/academic-service/internal/adapters/postgres"
	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

// drivers bundles the repositories a driver provides, because this service
// splits them one per aggregate.
type drivers struct {
	years     yearRepo
	terms     ports.TermRepository
	classes   ports.ClassRepository
	subjects  ports.SubjectRepository
	scales    ports.GradingScaleRepository
	timetable ports.TimetableRepository
}

type yearRepo interface {
	ports.AcademicYearRepository
	ports.LifecycleRepository
	ports.OutboxRepository
}

func tenantCtx(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

func date(t *testing.T, v string) domain.Date {
	t.Helper()
	d, err := domain.NewDate(v)
	if err != nil {
		t.Fatalf("new date %q: %v", v, err)
	}
	return d
}

func newYear(t *testing.T, tenantID string) *domain.AcademicYear {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.AcademicYear{
		ID: uuid.NewString(), TenantID: tenantID, Name: "2025/2026",
		Code: "Y-" + uuid.NewString()[:8], StartDate: date(t, "2025-09-01"),
		EndDate: date(t, "2026-07-31"), Status: "active",
		CreatedAt: now, UpdatedAt: now,
	}
}

func runAcademicContract(t *testing.T, d drivers) {
	const tenant = "upshs"
	const other = "aboom"
	ctx := tenantCtx(tenant)

	t.Run("a year round trips and stays inside its tenant", func(t *testing.T) {
		y := newYear(t, tenant)
		if err := d.years.Create(ctx, tenant, y); err != nil {
			t.Fatalf("create year: %v", err)
		}
		got, err := d.years.GetByID(ctx, tenant, y.ID)
		if err != nil {
			t.Fatalf("get year: %v", err)
		}
		if got.Code != y.Code || got.StartDate.String() != "2025-09-01" {
			t.Fatalf("round trip changed the year: %+v", got)
		}
		if _, err := d.years.GetByID(tenantCtx(other), other, y.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant read returned %v; want ErrNotFound", err)
		}
		if err := d.years.Delete(tenantCtx(other), other, y.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant delete returned %v; want ErrNotFound", err)
		}
	})

	t.Run("a class is reachable by its class teacher", func(t *testing.T) {
		y := newYear(t, tenant)
		if err := d.years.Create(ctx, tenant, y); err != nil {
			t.Fatalf("create year: %v", err)
		}
		teacher := uuid.NewString()
		now := time.Now().UTC().Truncate(time.Second)
		c := &domain.Class{
			ID: uuid.NewString(), TenantID: tenant, Name: "JHS 1A",
			AcademicYearID: y.ID, ClassTeacherID: &teacher,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := d.classes.Create(ctx, tenant, c); err != nil {
			t.Fatalf("create class: %v", err)
		}
		ids, err := d.classes.ListIDsByTeacher(ctx, tenant, teacher)
		if err != nil || len(ids) != 1 || ids[0] != c.ID {
			t.Fatalf("class ids by teacher = %v err=%v", ids, err)
		}
		leaked, err := d.classes.ListIDsByTeacher(tenantCtx(other), other, teacher)
		if err == nil && len(leaked) != 0 {
			t.Fatalf("class ids leaked across tenants: %v", leaked)
		}
	})

	t.Run("subjects page by cursor without repeating", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		for range 3 {
			s := &domain.Subject{
				ID: uuid.NewString(), TenantID: tenant, Name: "Mathematics",
				CreatedAt: now, UpdatedAt: now,
			}
			if err := d.subjects.Create(ctx, tenant, s); err != nil {
				t.Fatalf("create subject: %v", err)
			}
		}
		seen := map[string]bool{}
		cursor := ""
		for range 20 {
			p, next, err := d.subjects.List(ctx, tenant, 2, cursor)
			if err != nil {
				t.Fatalf("list subjects: %v", err)
			}
			for _, s := range p {
				if s.TenantID != tenant {
					t.Fatalf("list leaked a subject owned by %q", s.TenantID)
				}
				if seen[s.ID] {
					t.Fatalf("the cursor repeated subject %s", s.ID)
				}
				seen[s.ID] = true
			}
			if next == "" {
				break
			}
			cursor = next
		}
		if len(seen) < 3 {
			t.Fatalf("paging saw %d subjects; want at least 3", len(seen))
		}
	})

	t.Run("a grading scale keeps its bands", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		scale := &domain.GradingScale{
			ID: uuid.NewString(), TenantID: tenant, Name: "WASSCE",
			Ranges: []domain.GradeRange{
				{Min: 80, Max: 100, Grade: "A1", Remark: "Excellent"},
				{Min: 70, Max: 79, Grade: "B2"},
			},
			CreatedAt: now, UpdatedAt: now,
		}
		if err := d.scales.Create(ctx, tenant, scale); err != nil {
			t.Fatalf("create grading scale: %v", err)
		}
		got, err := d.scales.GetByID(ctx, tenant, scale.ID)
		if err != nil {
			t.Fatalf("get grading scale: %v", err)
		}
		if len(got.Ranges) != 2 || got.Ranges[0].Grade != "A1" || got.Ranges[0].Remark != "Excellent" {
			t.Fatalf("grading bands changed in storage: %+v", got.Ranges)
		}
	})

	t.Run("an overlapping active period is refused", func(t *testing.T) {
		classID, termID, subjectID := timetableFixture(t, d, tenant)
		first := newEntry(tenant, classID, termID, subjectID, "08:00", "09:00")
		if err := d.timetable.Create(ctx, tenant, first); err != nil {
			t.Fatalf("create first period: %v", err)
		}

		clash := newEntry(tenant, classID, termID, subjectID, "08:30", "09:30")
		if err := d.timetable.Create(ctx, tenant, clash); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("an overlapping period was accepted: %v", err)
		}

		// Touching at the boundary is not an overlap.
		adjacent := newEntry(tenant, classID, termID, subjectID, "09:00", "10:00")
		if err := d.timetable.Create(ctx, tenant, adjacent); err != nil {
			t.Fatalf("an adjacent period was refused: %v", err)
		}

		// Another tenant's own timetable is unaffected by this one.
		otherClass, otherTerm, otherSubject := timetableFixture(t, d, other)
		twin := newEntry(other, otherClass, otherTerm, otherSubject, "08:30", "09:30")
		if err := d.timetable.Create(tenantCtx(other), other, twin); err != nil {
			t.Fatalf("another tenant was blocked by this tenant's timetable: %v", err)
		}
	})

	t.Run("the timetable filters by term and weekday", func(t *testing.T) {
		classID, termID, subjectID := timetableFixture(t, d, tenant)
		e := newEntry(tenant, classID, termID, subjectID, "11:00", "12:00")
		if err := d.timetable.Create(ctx, tenant, e); err != nil {
			t.Fatalf("create period: %v", err)
		}
		got, err := d.timetable.List(ctx, tenant, ports.TimetableFilter{
			ClassIDs: []string{classID}, TermID: termID, Weekday: e.Weekday, Limit: 50,
		})
		if err != nil || len(got) != 1 || got[0].ID != e.ID {
			t.Fatalf("timetable filter returned %d entries err=%v", len(got), err)
		}
		if got[0].StartTime != "11:00" || got[0].EndTime != "12:00" {
			t.Fatalf("times changed in storage: %s-%s", got[0].StartTime, got[0].EndTime)
		}
	})

	t.Run("a lifecycle commit persists the year and queues one event", func(t *testing.T) {
		drainAcademic(t, d.years)
		y := newYear(t, tenant)
		err := d.years.CommitAcademicLifecycle(ctx, tenant,
			ports.AcademicMutation{Kind: ports.AcademicMutationYearCreate, Year: y},
			"academic.year_created.v1", ports.YearEventData(y, nil))
		if err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}
		if _, err := d.years.GetByID(ctx, tenant, y.ID); err != nil {
			t.Fatalf("lifecycle commit did not persist the year: %v", err)
		}
		claimed, err := d.years.ClaimPendingAcademicEvents(ctx, 100)
		if err != nil || len(claimed) != 1 || claimed[0].EventType != "academic.year_created.v1" {
			t.Fatalf("claimed %d events: %+v err=%v", len(claimed), claimed, err)
		}
		if err := d.years.MarkAcademicEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("mark published: %v", err)
		}
		again, err := d.years.ClaimPendingAcademicEvents(ctx, 100)
		if err != nil || len(again) != 0 {
			t.Fatalf("a published event was claimed again: %d err=%v", len(again), err)
		}
	})

	t.Run("a lifecycle delete still emits its event", func(t *testing.T) {
		drainAcademic(t, d.years)
		y := newYear(t, tenant)
		if err := d.years.Create(ctx, tenant, y); err != nil {
			t.Fatalf("create year: %v", err)
		}
		err := d.years.CommitAcademicLifecycle(ctx, tenant,
			ports.AcademicMutation{Kind: ports.AcademicMutationYearDelete, Year: y},
			"academic.year_deleted.v1", ports.YearEventData(y, nil))
		if err != nil {
			t.Fatalf("commit delete: %v", err)
		}
		if _, err := d.years.GetByID(ctx, tenant, y.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a lifecycle-deleted year is still readable: %v", err)
		}
		claimed, err := d.years.ClaimPendingAcademicEvents(ctx, 100)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("delete event not queued: %d err=%v", len(claimed), err)
		}
		if err := d.years.MarkAcademicEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("mark published: %v", err)
		}
	})

	t.Run("a failed event is retried rather than dropped", func(t *testing.T) {
		drainAcademic(t, d.years)
		y := newYear(t, tenant)
		if err := d.years.CommitAcademicLifecycle(ctx, tenant,
			ports.AcademicMutation{Kind: ports.AcademicMutationYearCreate, Year: y},
			"academic.year_created.v1", ports.YearEventData(y, nil)); err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}
		claimed, err := d.years.ClaimPendingAcademicEvents(ctx, 100)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("claim: %d err=%v", len(claimed), err)
		}
		if err := d.years.MarkAcademicEventFailed(ctx, claimed[0].ID, "bus unavailable"); err != nil {
			t.Fatalf("mark failed: %v", err)
		}
		if err := d.years.MarkAcademicEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("a failed event could not be acknowledged later: %v", err)
		}
	})
}

func newEntry(tenantID, classID, termID, subjectID, start, end string) *domain.TimetableEntry {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.TimetableEntry{
		ID: uuid.NewString(), TenantID: tenantID, ClassID: classID, TermID: termID,
		SubjectID: subjectID, Weekday: 1, StartTime: start, EndTime: end,
		Status: "active", CreatedAt: now, UpdatedAt: now,
	}
}

// timetableFixture creates the year, term, class and subject a period refers to.
// PostgreSQL enforces those references with foreign keys; MongoDB does not, so
// the fixture has to satisfy the stricter driver for the contract to mean the
// same thing on both.
func timetableFixture(t *testing.T, d drivers, tenantID string) (classID, termID, subjectID string) {
	t.Helper()
	ctx := tenantCtx(tenantID)
	now := time.Now().UTC().Truncate(time.Second)

	y := newYear(t, tenantID)
	if err := d.years.Create(ctx, tenantID, y); err != nil {
		t.Fatalf("create year: %v", err)
	}
	term := &domain.Term{
		ID: uuid.NewString(), TenantID: tenantID, AcademicYearID: y.ID, Name: "Term 1",
		StartDate: date(t, "2025-09-01"), EndDate: date(t, "2025-12-19"),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := d.terms.Create(ctx, tenantID, term); err != nil {
		t.Fatalf("create term: %v", err)
	}
	class := &domain.Class{
		ID: uuid.NewString(), TenantID: tenantID, Name: "JHS 1A",
		AcademicYearID: y.ID, CreatedAt: now, UpdatedAt: now,
	}
	if err := d.classes.Create(ctx, tenantID, class); err != nil {
		t.Fatalf("create class: %v", err)
	}
	subject := &domain.Subject{
		ID: uuid.NewString(), TenantID: tenantID, Name: "Mathematics",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := d.subjects.Create(ctx, tenantID, subject); err != nil {
		t.Fatalf("create subject: %v", err)
	}
	return class.ID, term.ID, subject.ID
}

func drainAcademic(t *testing.T, repo yearRepo) {
	t.Helper()
	ctx := tenantCtx("upshs")
	for range 50 {
		claimed, err := repo.ClaimPendingAcademicEvents(ctx, 100)
		if err != nil {
			t.Fatalf("drain: %v", err)
		}
		if len(claimed) == 0 {
			return
		}
		for _, e := range claimed {
			if err := repo.MarkAcademicEventPublished(ctx, e.ID); err != nil {
				t.Fatalf("drain acknowledge: %v", err)
			}
		}
	}
}

func TestPostgresAcademicRepositoriesSatisfyTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	pg := testkit.NewPostgres(ctx, t, "../../migrations")
	runAcademicContract(t, drivers{
		years:     pgadapter.NewRepository(pg.DB),
		terms:     pgadapter.NewTermRepository(pg.DB),
		classes:   pgadapter.NewClassRepository(pg.DB),
		subjects:  pgadapter.NewSubjectRepository(pg.DB),
		scales:    pgadapter.NewGradingScaleRepository(pg.DB),
		timetable: pgadapter.NewTimetableRepository(pg.DB),
	})
}

func TestMongoAcademicRepositoriesSatisfyTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	mg := testkit.NewMongo(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, mg.Store); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	runAcademicContract(t, drivers{
		years:     mongoadapter.NewRepository(mg.Store),
		terms:     mongoadapter.NewTermRepository(mg.Store),
		classes:   mongoadapter.NewClassRepository(mg.Store),
		subjects:  mongoadapter.NewSubjectRepository(mg.Store),
		scales:    mongoadapter.NewGradingScaleRepository(mg.Store),
		timetable: mongoadapter.NewTimetableRepository(mg.Store),
	})
}
