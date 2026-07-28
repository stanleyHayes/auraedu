import { Users } from "lucide-react";
import { PageHeader, DataTable, EmptyState, StatCard, Reveal } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClientForTenant } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { drilldownQuery, nextPageHref } from "@/lib/superadmin-drilldown";
import { SuperadminPagination } from "@/components/superadmin-pagination";

type Staff = OpenAPI.staff_v1.components["schemas"]["Staff"];

interface TenantStaffPageProps {
  params: Promise<{ code: string }>;
  searchParams: Promise<{ cursor?: string }>;
}

export default async function TenantStaffPage({ params, searchParams }: TenantStaffPageProps) {
  const { code } = await params;
  const { cursor } = await searchParams;
  await requireAuth();

  let staff: Staff[] = [];
  let nextCursor: string | null = null;
  let error: string | null = null;

  try {
    const client = await createServerClientForTenant(code);
    const res = await client.get<{ data?: Staff[]; next_cursor?: string | null }>(
      `/api/v1/staff?${drilldownQuery({}, cursor)}`,
    );
    staff = res.data ?? [];
    nextCursor = res.next_cursor ?? null;
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load staff";
  }

  const teachers = staff.filter((s) => s.staff_type === "teacher").length;
  const nextHref = nextPageHref(`/superadmin/tenants/${code}/staff`, {}, nextCursor);

  return (
    <div className="space-y-6">
      <Reveal>
        <PageHeader
          icon={<Users className="size-7" />}
          title="Staff"
          description={`Read-only staff directory for ${code}.`}
        />
      </Reveal>

      {error ? (
        <EmptyState
          title="Staff unavailable"
          description={error}
          icon={<Users className="size-8" />}
        />
      ) : (
        <>
          <Reveal delay={60}>
            <section className="grid gap-4 sm:grid-cols-3">
              <StatCard
                label="Staff"
                value={`${staff.length}${nextCursor ? "+" : ""}`}
                unit="this page"
              />
              <StatCard label="Teachers" value={teachers} unit="teaching" />
              <StatCard label="Support" value={staff.length - teachers} unit="non-teaching" />
            </section>
          </Reveal>

          <Reveal delay={100}>
            <DataTable
              caption={`Staff for tenant ${code}`}
              rows={staff}
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
                        {s.staff_code}
                      </div>
                    </div>
                  ),
                },
                {
                  key: "role",
                  header: "Role",
                  className: "w-32",
                  cell: (s) => (
                    <span className="rounded-full bg-[var(--muted)] px-2 py-0.5 text-xs capitalize">
                      {s.staff_type.replace("_", " ")}
                    </span>
                  ),
                },
                {
                  key: "contact",
                  header: "Contact",
                  cell: (s) => s.email ?? "—",
                },
                {
                  key: "status",
                  header: "Status",
                  className: "w-24",
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
              ]}
              empty={
                <EmptyState
                  title="No staff yet"
                  description="This school has not added any staff members."
                  icon={<Users className="size-8" />}
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
