import { GraduationCap } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

export default async function StaffPage() {
  await requireAuth();

  let staff: OpenAPI.staff_v1.components["schemas"]["Staff"][] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res =
      await client.get<OpenAPI.staff_v1.components["schemas"]["StaffList"]>(
        "/api/v1/staff?limit=50",
      );
    staff = res.data ?? [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load staff";
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<GraduationCap className="size-7" />}
        title="Staff"
        description="View and manage teaching and non-teaching staff."
      />

      {error ? (
        <EmptyState
          title="Could not load staff"
          description={error}
          icon={<GraduationCap className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Staff"
          rows={staff}
          keyExtractor={(s) => s.id}
          columns={[
            {
              key: "id",
              header: "ID",
              cell: (s) => (
                <span className="font-mono text-xs">{s.staff_code ?? s.id.slice(0, 8)}</span>
              ),
            },
            {
              key: "name",
              header: "Name",
              cell: (s) => `${s.first_name} ${s.last_name}`,
            },
            {
              key: "type",
              header: "Type",
              cell: (s) => <span className="capitalize">{s.staff_type.replace("_", " ")}</span>,
            },
            {
              key: "email",
              header: "Email",
              cell: (s) => s.email ?? "—",
            },
          ]}
          empty={
            <EmptyState
              title="No staff yet"
              description="Staff records will appear here once they are added."
              icon={<GraduationCap className="size-8" />}
            />
          }
        />
      )}
    </div>
  );
}
