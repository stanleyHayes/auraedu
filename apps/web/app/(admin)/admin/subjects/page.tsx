import { BookOpen } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { SubjectFormSheet } from "@/components/subject-form-sheet";
import { DeleteSubjectButton } from "@/components/delete-subject-button";

type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];

export default async function SubjectsPage() {
  await requireAuth();

  let subjects: Subject[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res = await client.get<OpenAPI.academic_v1.components["schemas"]["SubjectList"]>(
      "/api/v1/subjects?limit=50",
    );
    subjects = res.data ?? [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load subjects";
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<BookOpen className="size-7" />}
        title="Subjects"
        description="View and manage subjects offered by your school."
        action={<SubjectFormSheet mode="create" />}
      />

      {error ? (
        <EmptyState
          title="Could not load subjects"
          description={error}
          icon={<BookOpen className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Subjects"
          rows={subjects}
          keyExtractor={(s) => s.id}
          columns={[
            {
              key: "name",
              header: "Name",
              cell: (s) => <span className="font-medium">{s.name}</span>,
            },
            {
              key: "code",
              header: "Code",
              cell: (s) => (s.code ? <span className="font-mono text-xs">{s.code}</span> : "—"),
            },
            {
              key: "description",
              header: "Description",
              cell: (s) => s.description ?? "—",
            },
            {
              key: "actions",
              header: "Actions",
              className: "w-24",
              cell: (s) => (
                <div className="flex items-center gap-2">
                  <SubjectFormSheet mode="edit" initial={s} />
                  <DeleteSubjectButton id={s.id} name={s.name} />
                </div>
              ),
            },
          ]}
          empty={
            <EmptyState
              title="No subjects yet"
              description="Subjects will appear here once they are created."
              icon={<BookOpen className="size-8" />}
            />
          }
        />
      )}
    </div>
  );
}
