import { Clock3, Monitor, ShieldCheck } from "lucide-react";
import { EmptyState, PageHeader, Reveal } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";
import type { components as CbtComponents } from "@auraedu/shared-types/openapi/cbt.v1";

type Exam = CbtComponents["schemas"]["Exam"];

export default async function StudentCbtExamsPage() {
  let exams: Exam[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const list = await client.get<{ data?: Exam[]; next_cursor?: string | null }>(
      "/api/v1/cbt/exams",
    );
    exams = list.data ?? [];
  } catch {
    error = "Could not load CBT exams right now.";
  }

  const header = (
    <PageHeader
      eyebrow="Assessment workspace"
      icon={<Monitor className="size-7" />}
      title="Computer-based exams"
      description="See the tests your school has released, their timing and the conditions you need before you begin."
    />
  );

  if (error) {
    return (
      <div className="space-y-6">
        {header}
        <EmptyState
          icon={<Monitor className="size-8" />}
          title="CBT exams unavailable"
          description={error}
        />
      </div>
    );
  }

  if (exams.length === 0) {
    return (
      <div className="space-y-6">
        {header}
        <EmptyState
          icon={<Monitor className="size-8" />}
          title="No upcoming CBT exams"
          description="Your scheduled computer-based tests will appear here."
        />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {header}
      <ul className="grid gap-4 md:grid-cols-2" aria-label="Available computer-based exams">
        {exams.map((exam, index) => (
          <Reveal key={exam.id} delay={index * 45}>
            <li className="h-full overflow-hidden rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] transition-[border-color,box-shadow,transform] duration-300 hover:-translate-y-0.5 hover:border-[var(--color-brand)]/40 hover:shadow-lg">
              <div className="bg-[var(--color-navy)] p-5 text-white">
                <div className="flex items-start justify-between gap-4">
                  <span className="grid size-11 place-items-center rounded-2xl bg-white/10 text-[var(--color-signal)]">
                    <Monitor className="size-5" aria-hidden="true" />
                  </span>
                  <span className="rounded-full border border-white/15 bg-white/5 px-3 py-1 font-mono text-[10px] font-bold uppercase tracking-[0.14em] text-slate-300">
                    Secure session
                  </span>
                </div>
                <h2 className="mt-5 text-balance font-heading text-xl font-bold">{exam.title}</h2>
              </div>
              <div className="grid grid-cols-2 gap-px bg-[var(--border)]">
                <div className="bg-[var(--surface)] p-4">
                  <p className="text-xl font-black">{exam.question_ids?.length ?? 0}</p>
                  <p className="mt-1 text-xs text-[var(--muted-foreground)]">questions</p>
                </div>
                <div className="bg-[var(--surface)] p-4">
                  <p className="text-xl font-black">{exam.duration_minutes ?? 0}</p>
                  <p className="mt-1 text-xs text-[var(--muted-foreground)]">minutes</p>
                </div>
              </div>
              <div className="flex items-center justify-between gap-4 p-5 text-xs text-[var(--muted-foreground)]">
                <span className="inline-flex items-center gap-2 font-semibold">
                  <Clock3 className="size-4 text-[var(--primary)]" aria-hidden="true" />
                  {exam.start_at
                    ? new Date(exam.start_at).toLocaleString("en-GB")
                    : "Start time pending"}
                </span>
                <ShieldCheck
                  className="size-5 text-[var(--color-forest)]"
                  aria-label="School approved"
                />
              </div>
            </li>
          </Reveal>
        ))}
      </ul>
    </div>
  );
}
