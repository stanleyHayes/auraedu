import { School } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { ClassFormSheet } from "@/components/class-form-sheet";
import { DeleteClassButton } from "@/components/delete-class-button";

type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];
type Staff = OpenAPI.staff_v1.components["schemas"]["Staff"];

export default async function ClassesPage() {
  await requireAuth();

  let classes: AcademicClass[] = [];
  let years: AcademicYear[] = [];
  let staff: Staff[] = [];
  let error: string | null = null;

  const client = await createServerClient();

  try {
    const res = await client.get<OpenAPI.academic_v1.components["schemas"]["ClassList"]>(
      "/api/v1/classes?limit=50",
    );
    classes = res.data ?? [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load classes";
  }

  // Supplemental lookups are best-effort so the list still renders when a
  // related service or feature (e.g. staff_management) is unavailable.
  try {
    const res = await client.get<OpenAPI.academic_v1.components["schemas"]["AcademicYearList"]>(
      "/api/v1/academic-years?limit=50",
    );
    years = res.data ?? [];
  } catch {
    years = [];
  }

  try {
    const res = await client.get<OpenAPI.staff_v1.components["schemas"]["StaffList"]>(
      "/api/v1/staff?limit=100",
    );
    staff = res.data ?? [];
  } catch {
    staff = [];
  }

  const yearName = new Map(years.map((y) => [y.id, y.name]));
  const teacherName = new Map(staff.map((s) => [s.id, `${s.first_name} ${s.last_name}`]));

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<School className="size-7" />}
        title="Classes"
        description="View and manage classes for each academic year."
        action={<ClassFormSheet mode="create" years={years} staff={staff} />}
      />

      {error ? (
        <EmptyState
          title="Could not load classes"
          description={error}
          icon={<School className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Classes"
          rows={classes}
          keyExtractor={(c) => c.id}
          columns={[
            {
              key: "name",
              header: "Name",
              cell: (c) => <span className="font-medium">{c.name}</span>,
            },
            {
              key: "year",
              header: "Academic year",
              cell: (c) => yearName.get(c.academic_year_id) ?? "—",
            },
            {
              key: "teacher",
              header: "Class teacher",
              cell: (c) => (c.class_teacher_id ? (teacherName.get(c.class_teacher_id) ?? "—") : "—"),
            },
            {
              key: "capacity",
              header: "Capacity",
              cell: (c) => c.capacity ?? "—",
            },
            {
              key: "actions",
              header: "Actions",
              className: "w-24",
              cell: (c) => (
                <div className="flex items-center gap-2">
                  <ClassFormSheet mode="edit" initial={c} years={years} staff={staff} />
                  <DeleteClassButton id={c.id} name={c.name} />
                </div>
              ),
            },
          ]}
          empty={
            <EmptyState
              title="No classes yet"
              description="Classes will appear here once they are created."
              icon={<School className="size-8" />}
            />
          }
        />
      )}
    </div>
  );
}
