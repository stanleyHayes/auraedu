import { CalendarDays, Clock3, Flag, Orbit } from "lucide-react";
import { DataTable, EmptyState, PageHeader, Reveal, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { AcademicCalendarFormSheet } from "@/components/academic-calendar-form-sheet";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];
type Term = OpenAPI.academic_v1.components["schemas"]["Term"];

export default async function AcademicYearsPage() {
  await requireAuth();

  let years: AcademicYear[] = [];
  let terms: Term[] = [];
  let error: string | null = null;
  let termError: string | null = null;
  const client = await createServerClient();

  try {
    const response = await client.get<
      OpenAPI.academic_v1.components["schemas"]["AcademicYearList"]
    >("/api/v1/academic-years?limit=100");
    years = response.data ?? [];
  } catch (caught) {
    error = caught instanceof Error ? caught.message : "Failed to load academic years";
  }

  if (!error) {
    try {
      const response =
        await client.get<OpenAPI.academic_v1.components["schemas"]["TermList"]>(
          "/api/v1/terms?limit=100",
        );
      terms = response.data ?? [];
    } catch (caught) {
      termError = caught instanceof Error ? caught.message : "Failed to load terms";
    }
  }

  const current = years.find((year) => year.is_current);
  const yearNames = new Map(years.map((year) => [year.id, year.name]));
  const currentTerms = current ? terms.filter((term) => term.academic_year_id === current.id) : [];

  return (
    <div className="space-y-7">
      <PageHeader
        icon={<CalendarDays className="size-7" />}
        title="Academic calendar"
        description="Shape the years and teaching terms that every class, assessment, and learner journey depends on."
        action={
          <div className="flex flex-wrap gap-2">
            <AcademicCalendarFormSheet kind="term" mode="create" years={years} />
            <AcademicCalendarFormSheet kind="year" mode="create" />
          </div>
        }
      />

      <section className="grid gap-4 sm:grid-cols-3">
        <Reveal>
          <StatCard
            label="Current cycle"
            value={current?.name ?? "Not set"}
            unit={current?.code}
            tone={current ? "ok" : "warn"}
          />
        </Reveal>
        <Reveal delay={70}>
          <StatCard label="Teaching terms" value={currentTerms.length} unit="in current year" />
        </Reveal>
        <Reveal delay={140}>
          <StatCard label="Calendar history" value={years.length} unit="academic years" />
        </Reveal>
      </section>

      {current ? (
        <Reveal delay={80}>
          <section className="relative overflow-hidden rounded-3xl border border-[var(--border)] bg-[var(--foreground)] p-6 text-[var(--background)] shadow-[0_24px_64px_rgba(6,22,49,0.16)] sm:p-8">
            <span
              aria-hidden="true"
              className="absolute -right-24 -top-28 size-80 rounded-full bg-[var(--portal-accent,var(--color-brand))]/35 blur-3xl"
            />
            <span
              aria-hidden="true"
              className="absolute bottom-0 left-1/3 h-px w-1/2 bg-gradient-to-r from-transparent via-[var(--portal-signal,var(--color-signal))] to-transparent"
            />
            <div className="relative grid gap-7 lg:grid-cols-[1fr_auto] lg:items-end">
              <div>
                <div className="inline-flex items-center gap-2 rounded-full border border-[var(--background)]/15 bg-[var(--background)]/5 px-3 py-1.5 font-mono text-[10px] font-bold uppercase tracking-[0.18em]">
                  <Orbit className="size-3.5 text-[var(--portal-signal,var(--color-signal))]" />
                  Current operating cycle
                </div>
                <h2 className="mt-5 font-heading text-3xl font-black tracking-tight sm:text-4xl">
                  {current.name}
                </h2>
                <p className="mt-3 max-w-2xl text-sm leading-7 text-[var(--background)]/68">
                  {friendlyDate(current.start_date)} to {friendlyDate(current.end_date)} ·{" "}
                  {currentTerms.length} configured {currentTerms.length === 1 ? "term" : "terms"}
                </p>
              </div>
              <AcademicCalendarFormSheet kind="year" mode="edit" initial={current} />
            </div>
          </section>
        </Reveal>
      ) : null}

      {error ? (
        <EmptyState
          title="Could not load the academic calendar"
          description={error}
          icon={<CalendarDays className="size-8" />}
        />
      ) : (
        <div className="grid gap-7 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
          <Reveal delay={120}>
            <CalendarPanel
              icon={<CalendarDays className="size-5" />}
              title="Academic years"
              description="The durable calendar boundaries for school operations."
            >
              <DataTable
                caption="Academic years"
                rows={years}
                keyExtractor={(year) => year.id}
                columns={[
                  {
                    key: "name",
                    header: "Year",
                    cell: (year) => (
                      <div>
                        <p className="font-semibold">{year.name}</p>
                        <p className="mt-0.5 font-mono text-[10px] uppercase tracking-wider text-[var(--muted-foreground)]">
                          {year.code}
                        </p>
                      </div>
                    ),
                  },
                  {
                    key: "window",
                    header: "Window",
                    cell: (year) => (
                      <span className="text-xs">
                        {shortDate(year.start_date)} → {shortDate(year.end_date)}
                      </span>
                    ),
                  },
                  {
                    key: "state",
                    header: "State",
                    cell: (year) => (
                      <span
                        className={`rounded-full px-2 py-1 text-[10px] font-bold uppercase tracking-wider ${year.is_current ? "bg-emerald-500/10 text-emerald-700" : "bg-[var(--muted)] text-[var(--muted-foreground)]"}`}
                      >
                        {year.is_current ? "Current" : year.status}
                      </span>
                    ),
                  },
                  {
                    key: "actions",
                    header: "",
                    className: "w-12",
                    cell: (year) => (
                      <AcademicCalendarFormSheet kind="year" mode="edit" initial={year} />
                    ),
                  },
                ]}
                empty={
                  <EmptyState
                    title="Build the first school calendar"
                    description="Create an academic year before adding classes or teaching terms."
                    icon={<CalendarDays className="size-8" />}
                  />
                }
              />
            </CalendarPanel>
          </Reveal>

          <Reveal delay={180}>
            <CalendarPanel
              icon={<Flag className="size-5" />}
              title="Teaching terms"
              description="Sequenced learning windows tied to an academic year."
            >
              {termError ? (
                <EmptyState
                  title="Terms are temporarily unavailable"
                  description={termError}
                  icon={<Clock3 className="size-8" />}
                />
              ) : (
                <DataTable
                  caption="Teaching terms"
                  rows={terms}
                  keyExtractor={(term) => term.id}
                  columns={[
                    {
                      key: "name",
                      header: "Term",
                      cell: (term) => (
                        <div>
                          <p className="font-semibold">{term.name}</p>
                          <p className="mt-0.5 text-xs text-[var(--muted-foreground)]">
                            {yearNames.get(term.academic_year_id) ?? "Unknown year"}
                          </p>
                        </div>
                      ),
                    },
                    {
                      key: "window",
                      header: "Teaching window",
                      cell: (term) => (
                        <span className="text-xs">
                          {shortDate(term.start_date)} → {shortDate(term.end_date)}
                        </span>
                      ),
                    },
                    {
                      key: "actions",
                      header: "",
                      className: "w-12",
                      cell: (term) => (
                        <AcademicCalendarFormSheet
                          kind="term"
                          mode="edit"
                          initial={term}
                          years={years}
                        />
                      ),
                    },
                  ]}
                  empty={
                    <EmptyState
                      title="No teaching terms yet"
                      description="Add a term to turn the school year into usable teaching periods."
                      icon={<Flag className="size-8" />}
                    />
                  }
                />
              )}
            </CalendarPanel>
          </Reveal>
        </div>
      )}
    </div>
  );
}

function CalendarPanel({
  icon,
  title,
  description,
  children,
}: {
  icon: React.ReactNode;
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <section className="h-full overflow-hidden rounded-3xl border border-[var(--border)] bg-[var(--surface)] shadow-[0_14px_42px_rgba(6,22,49,0.06)]">
      <header className="flex items-start gap-3 border-b border-[var(--border)] bg-[var(--muted)]/55 px-5 py-4">
        <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-[var(--portal-accent,var(--color-brand))]/10 text-[var(--portal-accent,var(--color-brand))]">
          {icon}
        </span>
        <div>
          <h2 className="font-heading text-lg font-bold">{title}</h2>
          <p className="mt-0.5 text-xs leading-5 text-[var(--muted-foreground)]">{description}</p>
        </div>
      </header>
      <div className="p-2 sm:p-4">{children}</div>
    </section>
  );
}

function friendlyDate(value: string): string {
  return new Intl.DateTimeFormat("en", {
    day: "numeric",
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  }).format(new Date(`${value}T00:00:00Z`));
}

function shortDate(value: string): string {
  return new Intl.DateTimeFormat("en", {
    day: "2-digit",
    month: "short",
    year: "2-digit",
    timeZone: "UTC",
  }).format(new Date(`${value}T00:00:00Z`));
}
