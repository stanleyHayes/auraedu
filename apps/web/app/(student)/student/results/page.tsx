import type { OpenAPI } from "@auraedu/shared-types";
import { Award, BookOpen, TrendingUp } from "lucide-react";
import { EmptyState, Reveal } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";

type Assessment = OpenAPI.assessment_v1.components["schemas"]["Assessment"];
type AssessmentList = OpenAPI.assessment_v1.components["schemas"]["AssessmentList"];
type Score = OpenAPI.assessment_v1.components["schemas"]["Score"];
type ScoreList = OpenAPI.assessment_v1.components["schemas"]["ScoreList"];
type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type SubjectList = OpenAPI.academic_v1.components["schemas"]["SubjectList"];

interface PublishedResult {
  assessment: Assessment;
  score: Score;
  maximum: number | null;
  percentage: number | null;
}

function resultFor(assessment: Assessment, score: Score): PublishedResult {
  const maximum = score.max_score ?? assessment.max_score ?? null;
  return {
    assessment,
    score,
    maximum,
    percentage: maximum && maximum > 0 ? (score.score / maximum) * 100 : null,
  };
}

export default async function StudentResultsPage() {
  let results: PublishedResult[] = [];
  let subjects: Record<string, Subject> = {};
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const assessmentList = await client.get<AssessmentList>(
      "/api/v1/assessments?status=published&limit=50",
    );
    const assessments = assessmentList.data ?? [];
    const scoreLists = await Promise.all(
      assessments.map((assessment) =>
        client.get<ScoreList>(
          `/api/v1/assessments/${encodeURIComponent(assessment.id)}/scores?limit=100`,
        ),
      ),
    );
    results = assessments.flatMap((assessment, index) =>
      (scoreLists[index]?.data ?? []).map((score) => resultFor(assessment, score)),
    );
    try {
      const subjectList = await client.get<SubjectList>("/api/v1/subjects?limit=100");
      subjects = Object.fromEntries(
        (subjectList.data ?? []).map((subject) => [subject.id, subject]),
      );
    } catch {
      subjects = {};
    }
  } catch {
    error = "Your published results could not be loaded right now. Please try again shortly.";
  }

  if (error) {
    return (
      <EmptyState
        icon={<BookOpen className="size-8" />}
        title="Results unavailable"
        description={error}
      />
    );
  }

  if (results.length === 0) {
    return (
      <EmptyState
        icon={<BookOpen className="size-8" />}
        title="No published results"
        description="Your scores will appear here after your school publishes them."
      />
    );
  }

  const percentages = results.flatMap((result) =>
    result.percentage === null ? [] : [result.percentage],
  );
  const average =
    percentages.length > 0
      ? percentages.reduce((total, percentage) => total + percentage, 0) / percentages.length
      : null;

  return (
    <div className="space-y-7">
      <Reveal>
        <section className="relative overflow-hidden rounded-[var(--radius-lg)] bg-[var(--color-navy)] p-6 text-white sm:p-8">
          <div className="pointer-events-none absolute -right-20 -top-24 size-72 rounded-full bg-[var(--color-brand)]/25 blur-3xl" />
          <div className="relative grid gap-7 lg:grid-cols-[1fr_auto] lg:items-end">
            <div>
              <p className="font-mono text-xs font-bold uppercase tracking-[0.18em] text-[var(--color-signal)]">
                Published learning record
              </p>
              <h1 className="mt-3 font-heading text-3xl font-black tracking-tight sm:text-4xl">
                Your results
              </h1>
              <p className="mt-3 max-w-xl text-sm leading-6 text-slate-300">
                Only scores released by your school appear here. Draft assessments and other
                learners’ records remain private.
              </p>
            </div>
            <div className="grid grid-cols-2 gap-px overflow-hidden rounded-xl bg-white/15 text-center">
              <div className="bg-white/10 px-6 py-3">
                <strong className="block text-2xl">{results.length}</strong>
                <span className="text-xs text-slate-300">published</span>
              </div>
              <div className="bg-white/10 px-6 py-3">
                <strong className="block text-2xl">
                  {average === null ? "—" : `${Math.round(average)}%`}
                </strong>
                <span className="text-xs text-slate-300">simple average</span>
              </div>
            </div>
          </div>
        </section>
      </Reveal>

      <section className="grid gap-4 md:grid-cols-2" aria-label="Published results">
        {results.map((result, index) => {
          const subject = subjects[result.assessment.subject_id];
          const percentage = result.percentage === null ? null : Math.round(result.percentage);
          return (
            <Reveal key={`${result.assessment.id}-${result.score.id}`} delay={index * 45}>
              <article className="h-full rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5 transition-[border-color,box-shadow,transform] duration-300 hover:-translate-y-0.5 hover:border-[var(--color-brand)]/40 hover:shadow-lg">
                <div className="flex items-start justify-between gap-5">
                  <div className="min-w-0">
                    <p className="text-xs font-bold uppercase tracking-[0.13em] text-[var(--primary)]">
                      {subject?.name ?? result.assessment.type}
                    </p>
                    <h2 className="mt-2 truncate font-heading text-lg font-bold">
                      {result.assessment.name}
                    </h2>
                    <p className="mt-1 text-sm capitalize text-[var(--muted-foreground)]">
                      {result.assessment.type}
                    </p>
                  </div>
                  <div className="shrink-0 text-right">
                    <p className="font-heading text-2xl font-black tabular-nums">
                      {result.score.score}
                      <span className="text-sm font-semibold text-[var(--muted-foreground)]">
                        {result.maximum === null ? "" : ` / ${result.maximum}`}
                      </span>
                    </p>
                    <p className="mt-1 text-xs font-bold text-[var(--muted-foreground)]">
                      {percentage === null ? "Maximum unavailable" : `${percentage}%`}
                    </p>
                  </div>
                </div>
                {percentage !== null ? (
                  <div className="mt-5">
                    <div className="h-2 overflow-hidden rounded-full bg-[var(--muted)]">
                      <div
                        className="h-full rounded-full bg-gradient-to-r from-[var(--color-brand)] to-[var(--color-forest)]"
                        style={{ width: `${Math.min(100, Math.max(0, percentage))}%` }}
                      />
                    </div>
                    <div className="mt-3 flex items-center gap-2 text-xs text-[var(--muted-foreground)]">
                      {percentage >= 50 ? (
                        <Award className="size-4 text-[var(--color-forest)]" aria-hidden="true" />
                      ) : (
                        <TrendingUp className="size-4 text-[var(--primary)]" aria-hidden="true" />
                      )}
                      <span>Recorded against the published assessment maximum.</span>
                    </div>
                  </div>
                ) : null}
              </article>
            </Reveal>
          );
        })}
      </section>
    </div>
  );
}
