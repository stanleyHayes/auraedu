import { Scale } from "lucide-react";
import { EmptyState, PageHeader, Reveal, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { AdminGradingScaleDelete } from "@/components/admin-grading-scale-delete";
import { AdminGradingScaleSheet } from "@/components/admin-grading-scale-sheet";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

type GradingScale = OpenAPI.academic_v1.components["schemas"]["GradingScale"];
type GradeRange = OpenAPI.academic_v1.components["schemas"]["GradeRange"];
type GradingScaleList = OpenAPI.academic_v1.components["schemas"]["GradingScaleList"];

function sortedRanges(scale: GradingScale): GradeRange[] {
  return [...((scale.ranges ?? []) as GradeRange[])].sort((a, b) => b.max - a.max);
}

export default async function AdminGradingPage() {
  await requireAuth();

  let scales: GradingScale[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res = await client.get<GradingScaleList>("/api/v1/grading-scales");
    scales = res.data ?? [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load grading scales";
  }

  const bandCount = scales.reduce((sum, scale) => sum + (scale.ranges?.length ?? 0), 0);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Scale className="size-7" />}
        title="Grading scales"
        description="The grade bands assessments and report cards grade against. Bands must not overlap — every score lands in exactly one band."
        action={<AdminGradingScaleSheet mode="create" />}
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Grading scales" value={scales.length} unit="policies" />
        <StatCard label="Grade bands" value={bandCount} unit="defined" />
        <StatCard
          label="Bands per scale"
          value={scales.length > 0 ? (bandCount / scales.length).toFixed(1) : 0}
          unit="average"
        />
      </div>

      {error ? (
        <EmptyState
          title="Could not load grading scales"
          description={error}
          icon={<Scale className="size-8" />}
        />
      ) : scales.length === 0 ? (
        <EmptyState
          title="No grading scales yet"
          description="Create the first scale above; assessments and report cards grade against these bands."
          icon={<Scale className="size-8" />}
        />
      ) : (
        <Reveal className="grid gap-4 xl:grid-cols-2">
          {scales.map((scale) => {
            const ranges = sortedRanges(scale);
            return (
              <article
                key={scale.id}
                className="group rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-5 shadow-sm transition hover:-translate-y-0.5 hover:border-primary/35 hover:shadow-md"
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="min-w-0">
                    <h3 className="truncate font-heading text-lg font-bold">{scale.name}</h3>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {ranges.length} band{ranges.length === 1 ? "" : "s"} · highest{" "}
                      {ranges[0]?.grade ?? "—"} from {ranges[0]?.min ?? "—"}
                    </p>
                  </div>
                  <div className="flex shrink-0 items-center gap-1">
                    <AdminGradingScaleSheet mode="edit" initial={scale} />
                    <AdminGradingScaleDelete id={scale.id} name={scale.name} />
                  </div>
                </div>
                <div className="mt-4 flex flex-wrap gap-2 border-t border-border pt-4">
                  {ranges.map((range) => (
                    <span
                      key={`${range.grade}-${range.min}-${range.max}`}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-background/70 px-2.5 py-1 text-xs"
                    >
                      <span className="font-bold">{range.grade}</span>
                      <span className="font-mono text-muted-foreground">
                        {range.min}–{range.max}
                      </span>
                      {range.remark ? (
                        <span className="text-muted-foreground">· {range.remark}</span>
                      ) : null}
                    </span>
                  ))}
                </div>
              </article>
            );
          })}
        </Reveal>
      )}
    </div>
  );
}
