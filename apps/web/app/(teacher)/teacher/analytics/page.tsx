import { BarChart3, TrendingUp, Users, ClipboardCheck } from "lucide-react";
import { PageHeader, StatCard } from "@auraedu/ui";

export default function TeacherAnalyticsPage() {
  return (
    <div className="space-y-6">
      <PageHeader
        icon={<BarChart3 className="size-6" />}
        title="Analytics"
        description="Class performance and engagement metrics."
      />
      <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="Class average" value="—" unit="%" />
        <StatCard label="Attendance rate" value="—" unit="%" tone="ok" />
        <StatCard label="Assignments submitted" value="—" unit="students" />
        <StatCard label="Scores recorded" value="—" unit="records" />
      </section>
      <section className="grid gap-6 md:grid-cols-3">
        <div className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5 md:col-span-2">
          <h3 className="font-sans font-semibold tracking-tight">Performance over time</h3>
          <div className="mt-8 flex h-48 items-end justify-around gap-2">
            {[40, 55, 45, 70, 60, 75, 65].map((h, i) => (
              <div
                key={i}
                className="w-full rounded-t bg-[var(--primary)]/20"
                style={{ height: `${h}%` }}
                aria-hidden="true"
              />
            ))}
          </div>
          <p className="mt-4 text-center text-xs text-[var(--muted-foreground)]">
            Chart placeholder — real data will load once the analytics service is wired.
          </p>
        </div>
        <div className="space-y-4">
          <div className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
            <div className="flex items-center gap-2 text-[var(--muted-foreground)]">
              <Users className="size-4" aria-hidden="true" />
              <span className="text-sm font-medium">Students tracked</span>
            </div>
            <div className="mt-2 text-2xl font-extrabold tracking-tight">—</div>
          </div>
          <div className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
            <div className="flex items-center gap-2 text-[var(--muted-foreground)]">
              <ClipboardCheck className="size-4" aria-hidden="true" />
              <span className="text-sm font-medium">Assessments marked</span>
            </div>
            <div className="mt-2 text-2xl font-extrabold tracking-tight">—</div>
          </div>
          <div className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
            <div className="flex items-center gap-2 text-[var(--muted-foreground)]">
              <TrendingUp className="size-4" aria-hidden="true" />
              <span className="text-sm font-medium">Improvement</span>
            </div>
            <div className="mt-2 text-2xl font-extrabold tracking-tight">—</div>
          </div>
        </div>
      </section>
    </div>
  );
}
