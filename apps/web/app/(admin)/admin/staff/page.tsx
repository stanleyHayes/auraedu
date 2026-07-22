import { GraduationCap, UsersRound } from "lucide-react";
import { PageHeader, DataTable, EmptyState, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { StaffAssignmentWorkspace } from "@/components/staff-assignment-workspace";
import { StaffFormSheet } from "@/components/staff-form-sheet";

type Staff = OpenAPI.staff_v1.components["schemas"]["Staff"];
type Assignment = OpenAPI.staff_v1.components["schemas"]["StaffAssignment"];
type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type User = OpenAPI.identity_v1.components["schemas"]["User"];

export default async function StaffPage() {
  await requireAuth();

  let staff: Staff[] = [];
  let assignments: Assignment[] = [];
  let classes: AcademicClass[] = [];
  let subjects: Subject[] = [];
  let users: User[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res =
      await client.get<OpenAPI.staff_v1.components["schemas"]["StaffList"]>(
        "/api/v1/staff?limit=50",
      );
    staff = res.data ?? [];
    const [classResult, subjectResult, userResult] = await Promise.allSettled([
      client.get<OpenAPI.academic_v1.components["schemas"]["ClassList"]>(
        "/api/v1/classes?limit=100",
      ),
      client.get<OpenAPI.academic_v1.components["schemas"]["SubjectList"]>(
        "/api/v1/subjects?limit=100",
      ),
      client.get<OpenAPI.identity_v1.components["schemas"]["UserList"]>("/api/v1/users"),
    ]);
    classes = classResult.status === "fulfilled" ? (classResult.value.data ?? []) : [];
    subjects = subjectResult.status === "fulfilled" ? (subjectResult.value.data ?? []) : [];
    users = userResult.status === "fulfilled" ? userResult.value.data : [];
    const teacherAssignments = await Promise.allSettled(
      staff
        .filter((member) => member.staff_type === "teacher")
        .map((member) =>
          client.get<OpenAPI.staff_v1.components["schemas"]["StaffAssignmentList"]>(
            `/api/v1/staff/${encodeURIComponent(member.id)}/assignments?limit=100`,
          ),
        ),
    );
    assignments = teacherAssignments.flatMap((result) =>
      result.status === "fulfilled" ? (result.value.data ?? []) : [],
    );
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load staff";
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<UsersRound className="size-7" />}
        title="People & teaching scope"
        description="Shape the team, then connect every teacher to the exact learning spaces they lead."
        action={<StaffFormSheet mode="create" users={users} />}
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard
          label="Active team"
          value={staff.filter((member) => member.status === "active").length}
          unit="people"
        />
        <StatCard
          label="Teachers"
          value={staff.filter((member) => member.staff_type === "teacher").length}
          unit="teaching profiles"
        />
        <StatCard
          label="Scope links"
          value={assignments.length}
          unit="class assignments"
          tone="ok"
        />
      </div>

      {!error ? (
        <StaffAssignmentWorkspace
          teachers={staff.filter((member) => member.staff_type === "teacher")}
          assignments={assignments}
          classes={classes}
          subjects={subjects}
        />
      ) : null}

      {error ? (
        <EmptyState
          title="Could not load staff"
          description={error}
          icon={<GraduationCap className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Staff directory"
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
              key: "status",
              header: "Status",
              cell: (s) => (
                <span
                  className={
                    s.status === "active"
                      ? "font-bold text-emerald-700"
                      : "font-bold text-[var(--muted-foreground)]"
                  }
                >
                  {s.status === "active" ? "Active" : "Inactive"}
                </span>
              ),
            },
            {
              key: "email",
              header: "Email",
              cell: (s) => s.email ?? "—",
            },
            {
              key: "actions",
              header: "Actions",
              className: "w-20",
              cell: (s) => <StaffFormSheet mode="edit" initial={s} users={users} />,
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
