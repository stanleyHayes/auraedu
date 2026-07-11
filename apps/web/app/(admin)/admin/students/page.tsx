import { Users } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

export default async function StudentsPage() {
  await requireAuth();

  let students: OpenAPI.student_v1.components["schemas"]["Student"][] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res = await client.get<OpenAPI.student_v1.components["schemas"]["StudentList"]>("/api/v1/students?limit=50");
    students = res.data ?? [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load students";
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Users className="size-7" />}
        title="Students"
        description="View and manage students enrolled in your school."
      />

      {error ? (
        <EmptyState
          title="Could not load students"
          description={error}
          icon={<Users className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Students"
          rows={students}
          keyExtractor={(s) => s.id}
          columns={[
            {
              key: "id",
              header: "ID",
              cell: (s) => <span className="font-mono text-xs">{s.student_code}</span>,
            },
            {
              key: "name",
              header: "Name",
              cell: (s) => `${s.first_name} ${s.last_name}`,
            },
            {
              key: "status",
              header: "Status",
              cell: (s) => <span className="capitalize">{s.status ?? "active"}</span>,
            },
          ]}
          empty={
            <EmptyState
              title="No students yet"
              description="Students will appear here once records are added."
              icon={<Users className="size-8" />}
            />
          }
        />
      )}
    </div>
  );
}
