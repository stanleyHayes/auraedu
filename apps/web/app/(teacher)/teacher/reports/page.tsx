import { FileText, GraduationCap } from "lucide-react";
import { PageHeader } from "@auraedu/ui";

const REPORT_LINKS = [
  { label: "Class report cards", description: "End-of-term report cards by class.", href: "#" },
  { label: "Student transcripts", description: "Individual academic transcripts.", href: "#" },
  { label: "Attendance reports", description: "Termly attendance summaries.", href: "#" },
];

export default function TeacherReportsPage() {
  return (
    <div className="space-y-6">
      <PageHeader
        icon={<FileText className="size-6" />}
        title="Reports"
        description="Generate and review school reports."
      />
      <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {REPORT_LINKS.map((report) => (
          <div
            key={report.label}
            className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5"
          >
            <div className="flex items-start gap-3">
              <span
                aria-hidden="true"
                className="grid size-10 flex-none place-items-center rounded-[var(--radius-lg)] bg-[var(--accent)] text-[var(--primary)]"
              >
                <GraduationCap className="size-5" />
              </span>
              <div className="min-w-0 flex-1">
                <h3 className="font-display font-semibold tracking-tight">{report.label}</h3>
                <p className="mt-1 text-sm text-[var(--muted-foreground)]">{report.description}</p>
              </div>
            </div>
            <a
              href={report.href}
              className="mt-4 inline-flex h-10 w-full items-center justify-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--surface)] px-4 text-sm font-bold text-[var(--foreground)] transition-colors hover:bg-[var(--muted)]"
            >
              Open reports
            </a>
          </div>
        ))}
      </section>
    </div>
  );
}
