import { Users } from "lucide-react";
import { PageHeader, DataTable, EmptyState, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { StudentFormSheet } from "@/components/student-form-sheet";

type Student = OpenAPI.student_v1.components["schemas"]["Student"];
type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];
type User = OpenAPI.identity_v1.components["schemas"]["User"];

export default async function StudentsPage() {
  await requireAuth();

  let students: Student[] = [];
  let classes: AcademicClass[] = [];
  let years: AcademicYear[] = [];
  let users: User[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res = await client.get<OpenAPI.student_v1.components["schemas"]["StudentList"]>(
      "/api/v1/students?limit=50",
    );
    students = res.data ?? [];
    const [classResult, yearResult, userResult] = await Promise.allSettled([
      client.get<OpenAPI.academic_v1.components["schemas"]["ClassList"]>(
        "/api/v1/classes?limit=100",
      ),
      client.get<OpenAPI.academic_v1.components["schemas"]["AcademicYearList"]>(
        "/api/v1/academic-years?limit=100",
      ),
      client.get<OpenAPI.identity_v1.components["schemas"]["UserList"]>("/api/v1/users"),
    ]);
    classes = classResult.status === "fulfilled" ? (classResult.value.data ?? []) : [];
    years = yearResult.status === "fulfilled" ? (yearResult.value.data ?? []) : [];
    users =
      userResult.status === "fulfilled"
        ? userResult.value.data.filter(
            (user) => user.status === "active" && user.role === "student",
          )
        : [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load students";
  }

  const classNames = new Map(classes.map((item) => [item.id, item.name]));

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Users className="size-7" />}
        title="Students"
        description="Welcome learners, establish their first enrolment, and keep every lifecycle state honest."
        action={<StudentFormSheet mode="create" classes={classes} years={years} users={users} />}
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard
          label="Active learners"
          value={students.filter((student) => student.status === "active").length}
          unit="records"
        />
        <StatCard
          label="Current classes"
          value={new Set(students.map((student) => student.class_id).filter(Boolean)).size}
          unit="represented"
        />
        <StatCard
          label="Portal-linked"
          value={students.filter((student) => student.user_id).length}
          unit="accounts"
          tone="ok"
        />
      </div>

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
              key: "class",
              header: "Current class",
              cell: (s) =>
                s.class_id ? (classNames.get(s.class_id) ?? "Class unavailable") : "Not enrolled",
            },
            {
              key: "status",
              header: "Status",
              cell: (s) => <span className="capitalize">{s.status ?? "active"}</span>,
            },
            {
              key: "actions",
              header: "Actions",
              className: "w-20",
              cell: (s) => (
                <StudentFormSheet
                  mode="edit"
                  initial={s}
                  classes={classes}
                  years={years}
                  users={users}
                />
              ),
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
