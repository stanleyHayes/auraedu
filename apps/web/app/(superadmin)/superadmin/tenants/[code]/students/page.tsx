import { GraduationCap } from "lucide-react";
import { PageHeader, DataTable, EmptyState, StatCard, Reveal } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClientForTenant } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { drilldownQuery, nextPageHref, formatDateTime } from "@/lib/superadmin-drilldown";
import { SuperadminPagination } from "@/components/superadmin-pagination";

type Student = OpenAPI.student_v1.components["schemas"]["Student"];
type SchoolClass = OpenAPI.academic_v1.components["schemas"]["Class"];

interface TenantStudentsPageProps {
  params: Promise<{ code: string }>;
  searchParams: Promise<{ cursor?: string }>;
}

export default async function TenantStudentsPage({
  params,
  searchParams,
}: TenantStudentsPageProps) {
  const { code } = await params;
  const { cursor } = await searchParams;
  await requireAuth();

  let students: Student[] = [];
  let nextCursor: string | null = null;
  let classNames = new Map<string, string>();
  let error: string | null = null;

  try {
    const client = await createServerClientForTenant(code);
    const [studentPage, classPage] = await Promise.all([
      client.get<{ data?: Student[]; next_cursor?: string | null }>(
        `/api/v1/students?${drilldownQuery({}, cursor)}`,
      ),
      // Class names are display sugar — the roster still renders when the lookup fails.
      client
        .get<{ data?: SchoolClass[] }>("/api/v1/classes?limit=100")
        .catch(() => ({ data: [] as SchoolClass[] })),
    ]);
    students = studentPage.data ?? [];
    nextCursor = studentPage.next_cursor ?? null;
    classNames = new Map((classPage.data ?? []).map((c) => [c.id, c.name]));
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load students";
  }

  const active = students.filter((s) => s.status === "active").length;
  const nextHref = nextPageHref(`/superadmin/tenants/${code}/students`, {}, nextCursor);

  return (
    <div className="space-y-6">
      <Reveal>
        <PageHeader
          icon={<GraduationCap className="size-7" />}
          title="Students"
          description={`Read-only enrolment roster for ${code}.`}
        />
      </Reveal>

      {error ? (
        <EmptyState
          title="Students unavailable"
          description={error}
          icon={<GraduationCap className="size-8" />}
        />
      ) : (
        <>
          <Reveal delay={60}>
            <section className="grid gap-4 sm:grid-cols-3">
              <StatCard
                label="Students"
                value={`${students.length}${nextCursor ? "+" : ""}`}
                unit="this page"
              />
              <StatCard label="Active" value={active} unit="enrolled" tone="ok" />
              <StatCard
                label="Inactive"
                value={students.length - active}
                unit="withdrawn / graduated"
              />
            </section>
          </Reveal>

          <Reveal delay={100}>
            <DataTable
              caption={`Students for tenant ${code}`}
              rows={students}
              keyExtractor={(s) => s.id}
              columns={[
                {
                  key: "name",
                  header: "Name",
                  cell: (s) => (
                    <div>
                      <div className="font-medium">
                        {s.first_name} {s.last_name}
                      </div>
                      <div className="mt-0.5 font-mono text-xs text-[var(--muted-foreground)]">
                        {s.student_code}
                      </div>
                    </div>
                  ),
                },
                {
                  key: "class",
                  header: "Class",
                  cell: (s) => (s.class_id ? (classNames.get(s.class_id) ?? "—") : "—"),
                },
                {
                  key: "status",
                  header: "Status",
                  className: "w-28",
                  cell: (s) =>
                    s.status === "active" ? (
                      <span className="rounded-full bg-[var(--color-ok)]/10 px-2 py-0.5 text-xs capitalize text-[var(--color-ok)]">
                        {s.status}
                      </span>
                    ) : (
                      <span className="rounded-full bg-[var(--muted)] px-2 py-0.5 text-xs capitalize text-[var(--muted-foreground)]">
                        {s.status}
                      </span>
                    ),
                },
                {
                  key: "created",
                  header: "Enrolled",
                  cell: (s) => (
                    <span className="font-mono text-xs">{formatDateTime(s.created_at)}</span>
                  ),
                },
              ]}
              empty={
                <EmptyState
                  title="No students yet"
                  description="This school has not enrolled any students."
                  icon={<GraduationCap className="size-8" />}
                />
              }
            />
          </Reveal>

          <SuperadminPagination nextHref={nextHref} />
        </>
      )}
    </div>
  );
}
