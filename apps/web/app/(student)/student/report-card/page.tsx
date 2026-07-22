import { ArrowDownToLine, FileText, ShieldCheck } from "lucide-react";
import { EmptyState, PageHeader, Reveal } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

type ReportCard = OpenAPI.report_v1.components["schemas"]["ReportCard"];

export default async function StudentReportCardPage() {
  let cards: ReportCard[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    // Report Service resolves the identity actor to the canonical learner ID
    // and injects the published-only scope server-side.
    const list = await client.get<{ data?: ReportCard[]; next_cursor?: string | null }>(
      "/api/v1/report-cards",
    );
    cards = list.data ?? [];
  } catch {
    error = "Could not load report cards right now.";
  }

  const header = (
    <PageHeader
      eyebrow="Official learning record"
      icon={<FileText className="size-7" />}
      title="Report cards"
      description="Published term records from your school, ready to review or keep as a PDF."
    />
  );

  if (error) {
    return (
      <div className="space-y-6">
        {header}
        <EmptyState
          icon={<FileText className="size-8" />}
          title="Report cards unavailable"
          description={error}
        />
      </div>
    );
  }

  if (cards.length === 0) {
    return (
      <div className="space-y-6">
        {header}
        <EmptyState
          icon={<FileText className="size-8" />}
          title="No report cards yet"
          description="Your term report cards will appear here once they are published."
        />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {header}
      <ul className="grid gap-4 lg:grid-cols-2" aria-label="Published report cards">
        {cards.map((card, index) => (
          <Reveal key={card.id} delay={index * 45}>
            <li className="group relative h-full overflow-hidden rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5 transition-[border-color,box-shadow,transform] duration-300 hover:-translate-y-0.5 hover:border-[var(--color-brand)]/40 hover:shadow-lg">
              <span className="pointer-events-none absolute -right-10 -top-12 size-32 rounded-full bg-[var(--accent)]" />
              <div className="relative flex items-start gap-4">
                <span className="grid size-12 shrink-0 place-items-center rounded-2xl bg-[var(--color-navy)] text-[var(--color-signal)] shadow-lg">
                  <FileText className="size-5" aria-hidden="true" />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-2.5 py-1 text-[10px] font-black uppercase tracking-[0.12em] text-emerald-800">
                      <ShieldCheck className="size-3.5" aria-hidden="true" /> Published
                    </span>
                  </div>
                  <h2 className="mt-3 font-heading text-xl font-bold text-[var(--foreground)]">
                    Report card {card.term_id ? `· Term ${card.term_id}` : ""}
                  </h2>
                  <p className="mt-2 text-sm text-[var(--muted-foreground)]">
                    {card.generated_at
                      ? `Generated ${new Date(card.generated_at).toLocaleDateString("en-GB")}`
                      : "Prepared by your school"}
                  </p>
                </div>
              </div>
              {card.status === "published" ? (
                <a
                  href={`/api/reports/${card.id}/download`}
                  className="relative mt-5 inline-flex min-h-11 w-full items-center justify-center gap-2 rounded-[var(--radius-sm)] bg-[var(--primary)] px-4 text-sm font-bold text-[var(--primary-foreground)] transition-transform group-hover:-translate-y-0.5"
                >
                  <ArrowDownToLine className="size-4" aria-hidden="true" /> Download PDF
                </a>
              ) : null}
            </li>
          </Reveal>
        ))}
      </ul>
    </div>
  );
}
