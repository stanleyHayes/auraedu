import { FileText } from "lucide-react";
import { DataTable, EmptyState, PageHeader, Reveal, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { AdminReportTemplateSheet } from "@/components/admin-report-template-sheet";
import { AdminReportTemplateDeleteButton } from "@/components/admin-report-template-delete-button";
import { AdminReportCardActions } from "@/components/admin-report-card-actions";

const CARD_STATUS_STYLE: Record<string, string> = {
  published: "bg-emerald-50 text-emerald-800",
  generating: "bg-blue-50 text-blue-800",
  draft: "bg-amber-50 text-amber-800",
  archived: "bg-muted text-muted-foreground",
};

const TEMPLATE_STATUS_STYLE: Record<string, string> = {
  active: "bg-emerald-50 text-emerald-800",
  draft: "bg-amber-50 text-amber-800",
  archived: "bg-muted text-muted-foreground",
};

export default async function AdminReportsPage() {
  await requireAuth();
  const client = await createServerClient();

  const [cardResult, templateResult, yearResult, studentResult] = await Promise.allSettled([
    client.get<OpenAPI.report_v1.components["schemas"]["ReportCardList"]>(
      "/api/v1/report-cards?limit=100",
    ),
    client.get<OpenAPI.report_v1.components["schemas"]["ReportTemplateList"]>(
      "/api/v1/report-templates?limit=100",
    ),
    client.get<OpenAPI.academic_v1.components["schemas"]["AcademicYearList"]>(
      "/api/v1/academic-years?limit=50",
    ),
    client.get<OpenAPI.student_v1.components["schemas"]["StudentList"]>(
      "/api/v1/students?limit=100",
    ),
  ]);

  if (cardResult.status === "rejected" && templateResult.status === "rejected")
    return (
      <EmptyState
        icon={<FileText className="size-8" />}
        title="Report cards unavailable"
        description="The reporting service could not be reached."
      />
    );

  const rows = cardResult.status === "fulfilled" ? cardResult.value.data : [];
  const templates = templateResult.status === "fulfilled" ? templateResult.value.data : [];
  const years = yearResult.status === "fulfilled" ? (yearResult.value.data ?? []) : [];
  const students = studentResult.status === "fulfilled" ? (studentResult.value.data ?? []) : [];

  const yearName = new Map(years.map((year) => [year.id, year.name]));
  const studentName = new Map(
    students.map((student) => [
      student.id,
      `${student.first_name} ${student.last_name}`.trim() || student.student_code,
    ]),
  );
  const templateName = new Map(templates.map((template) => [template.id, template.name]));
  const count = (status: string) => rows.filter((card) => card.status === status).length;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<FileText className="size-6" />}
        title="Report cards"
        description="Design report templates, generate PDFs, and publish cards to guardians."
        action={<AdminReportTemplateSheet mode="create" years={years} />}
      />

      <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="Draft" value={count("draft")} unit="loaded cards" tone="warn" />
        <StatCard label="Generating" value={count("generating")} unit="PDF jobs" />
        <StatCard label="Published" value={count("published")} unit="ready" tone="ok" />
        <StatCard label="Archived" value={count("archived")} unit="retained" />
      </section>

      <Reveal delay={70}>
        <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <h2 className="mb-4 font-heading text-lg font-bold">Report templates</h2>
          {templateResult.status === "rejected" ? (
            <EmptyState
              icon={<FileText className="size-8" />}
              title="Could not load templates"
              description="Report templates could not be loaded. Cards remain visible below."
            />
          ) : (
            <DataTable
              caption="Report templates"
              rows={templates}
              keyExtractor={(template) => template.id}
              columns={[
                {
                  key: "name",
                  header: "Template",
                  cell: (template) => <span className="font-semibold">{template.name}</span>,
                },
                {
                  key: "year",
                  header: "Academic year",
                  cell: (template) => yearName.get(template.academic_year_id) ?? "—",
                },
                {
                  key: "status",
                  header: "Status",
                  cell: (template) => (
                    <span
                      className={`rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${
                        TEMPLATE_STATUS_STYLE[template.status] ?? "bg-muted text-muted-foreground"
                      }`}
                    >
                      {template.status}
                    </span>
                  ),
                },
                {
                  key: "updated",
                  header: "Updated",
                  cell: (template) => new Date(template.updated_at).toLocaleDateString("en-GB"),
                },
                {
                  key: "actions",
                  header: "Actions",
                  className: "w-24",
                  cell: (template) => (
                    <div className="flex items-center gap-2">
                      <AdminReportTemplateSheet mode="edit" initial={template} years={years} />
                      <AdminReportTemplateDeleteButton id={template.id} name={template.name} />
                    </div>
                  ),
                },
              ]}
              empty={
                <EmptyState
                  icon={<FileText className="size-8" />}
                  title="No report templates"
                  description="Create a template to define the layout of generated report cards."
                />
              }
            />
          )}
        </section>
      </Reveal>

      <Reveal delay={120}>
        <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <h2 className="mb-4 font-heading text-lg font-bold">Report cards</h2>
          {cardResult.status === "rejected" ? (
            <EmptyState
              icon={<FileText className="size-8" />}
              title="Could not load report cards"
              description="Report cards could not be loaded. Templates remain visible above."
            />
          ) : (
            <DataTable
              caption="Report cards"
              rows={rows}
              keyExtractor={(card) => card.id}
              columns={[
                {
                  key: "student",
                  header: "Student",
                  cell: (card) => (
                    <span className="font-semibold">
                      {studentName.get(card.student_id) ?? card.student_id}
                    </span>
                  ),
                },
                {
                  key: "template",
                  header: "Template",
                  cell: (card) =>
                    card.template_id ? (templateName.get(card.template_id) ?? "—") : "—",
                },
                {
                  key: "status",
                  header: "Status",
                  cell: (card) => (
                    <span
                      className={`rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${
                        CARD_STATUS_STYLE[card.status] ?? "bg-muted text-muted-foreground"
                      }`}
                    >
                      {card.status}
                    </span>
                  ),
                },
                {
                  key: "updated",
                  header: "Updated",
                  cell: (card) => new Date(card.updated_at).toLocaleDateString("en-GB"),
                },
                {
                  key: "actions",
                  header: "Actions",
                  cell: (card) => <AdminReportCardActions id={card.id} status={card.status} />,
                },
              ]}
              empty={
                <EmptyState
                  icon={<FileText className="size-8" />}
                  title="No report cards"
                  description="Report cards will appear once preparation begins."
                />
              }
            />
          )}
        </section>
      </Reveal>
    </div>
  );
}
