import { FileText } from "lucide-react";
import { EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { publicApiUrl } from "@auraedu/config";
import { createServerClient, getSession } from "@/lib/api";

// The live report-service DTO is ahead of the published contract: status and
// pdf_path/generated_at (set only once published) are not in report_v1 yet, and
// academic_year_id/template_id may be omitted on event-created drafts.
type ReportCard = OpenAPI.report_v1.components["schemas"]["ReportCard"] & {
  status?: string;
  academic_year_id?: string;
  template_id?: string;
  pdf_path?: string | null;
  created_at?: string;
  updated_at?: string;
};

export default async function StudentReportCardPage() {
  const session = await getSession();
  let cards: ReportCard[] = [];
  let error: string | null = null;

  // NOTE: student_id is the identity user id until the backend exposes an
  // actor→student-record mapping; without a session, fall through to the
  // empty state rather than listing tenant-wide cards.
  if (session?.user_id) {
    try {
      const client = await createServerClient();
      const list = await client.get<{ data?: ReportCard[]; next_cursor?: string | null }>(
        `/api/v1/report-cards?student_id=${session.user_id}`,
      );
      cards = list.data ?? [];
    } catch {
      error = "Could not load report cards right now.";
    }
  }

  if (error) {
    return (
      <EmptyState
        icon={<FileText className="size-8" />}
        title="Report cards unavailable"
        description={error}
      />
    );
  }

  if (cards.length === 0) {
    return (
      <EmptyState
        icon={<FileText className="size-8" />}
        title="No report cards yet"
        description="Your term report cards will appear here once they are published."
      />
    );
  }

  return (
    <div className="space-y-4">
      <h2 className="font-heading text-lg font-semibold tracking-tight">Report cards</h2>
      <ul className="space-y-3">
        {cards.map((card) => (
          <li
            key={card.id}
            className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-4"
          >
            <div className="flex items-start justify-between gap-4">
              <div>
                <h3 className="font-medium text-[var(--foreground)]">
                  Report card {card.term_id ? `· Term ${card.term_id}` : ""}
                </h3>
                <p className="mt-1 text-sm capitalize text-[var(--muted-foreground)]">
                  Status: {card.status ?? "draft"}
                  {card.generated_at
                    ? ` · Generated ${new Date(card.generated_at).toLocaleDateString()}`
                    : ""}
                </p>
              </div>
              {card.status === "published" ? (
                <a
                  href={`${publicApiUrl}/api/v1/report-cards/${card.id}/download`}
                  target="_blank"
                  rel="noreferrer"
                  className="shrink-0 text-sm font-medium text-[var(--primary)] hover:underline"
                >
                  Download PDF
                </a>
              ) : null}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
