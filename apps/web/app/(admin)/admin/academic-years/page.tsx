import { CalendarDays } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

export default async function AcademicYearsPage() {
  await requireAuth();

  let years: OpenAPI.academic_v1.components["schemas"]["AcademicYear"][] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res = await client.get<OpenAPI.academic_v1.components["schemas"]["AcademicYearList"]>("/api/v1/academic-years?limit=50");
    years = res.data ?? [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load academic years";
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<CalendarDays className="size-7" />}
        title="Academic years"
        description="View academic years and terms for your school."
      />

      {error ? (
        <EmptyState
          title="Could not load academic years"
          description={error}
          icon={<CalendarDays className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Academic years"
          rows={years}
          keyExtractor={(y) => y.id}
          columns={[
            {
              key: "name",
              header: "Name",
              cell: (y) => (
                <span className="font-medium">
                  {y.name}
                  {y.is_current ? (
                    <span className="ml-2 rounded-full bg-[var(--color-ok)]/10 px-2 py-0.5 text-xs text-[var(--color-ok)]">
                      Current
                    </span>
                  ) : null}
                </span>
              ),
            },
            {
              key: "start",
              header: "Start date",
              cell: (y) => y.start_date,
            },
            {
              key: "end",
              header: "End date",
              cell: (y) => y.end_date,
            },
          ]}
          empty={
            <EmptyState
              title="No academic years yet"
              description="Academic years will appear here once they are created."
              icon={<CalendarDays className="size-8" />}
            />
          }
        />
      )}
    </div>
  );
}
