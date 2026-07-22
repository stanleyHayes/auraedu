import { ShieldCheck, Sparkles } from "lucide-react";
import { EmptyState, PageHeader, Reveal } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

type Recommendation = OpenAPI.ai_recommendation_v1.components["schemas"]["Recommendation"];

export default async function StudentRecommendationsPage() {
  let recommendations: Recommendation[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    // The backend resolves the identity user to its learner record and exposes
    // only teacher-approved guidance; client-supplied student IDs are not trusted.
    const list = await client.get<
      OpenAPI.ai_recommendation_v1.components["schemas"]["RecommendationList"]
    >("/api/v1/ai/recommendations?status=approved");
    recommendations = list.data ?? [];
  } catch {
    error = "Could not load recommendations right now.";
  }

  const header = (
    <PageHeader
      eyebrow="Teacher-approved guidance"
      icon={<Sparkles className="size-7" />}
      title="Learning recommendations"
      description="Useful next steps suggested from learning evidence and released only after a teacher reviews them."
    />
  );

  if (error) {
    return (
      <div className="space-y-6">
        {header}
        <EmptyState
          icon={<Sparkles className="size-8" />}
          title="Recommendations unavailable"
          description={error}
        />
      </div>
    );
  }

  if (recommendations.length === 0) {
    return (
      <div className="space-y-6">
        {header}
        <EmptyState
          icon={<Sparkles className="size-8" />}
          title="No recommendations yet"
          description="Personalised learning recommendations will appear here once your teachers approve them."
        />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {header}
      <ul className="grid gap-4 md:grid-cols-2" aria-label="Approved learning recommendations">
        {recommendations.map((recommendation, index) => (
          <Reveal key={recommendation.id} delay={index * 45}>
            <li className="relative h-full overflow-hidden rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5 transition-[border-color,box-shadow,transform] duration-300 hover:-translate-y-0.5 hover:border-[var(--color-brand)]/40 hover:shadow-lg">
              <span className="pointer-events-none absolute -right-12 -top-14 size-36 rounded-full bg-[var(--accent)]" />
              <div className="relative flex items-start justify-between gap-4">
                <span className="grid size-11 shrink-0 place-items-center rounded-2xl bg-[var(--color-navy)] text-[var(--color-signal)]">
                  <Sparkles className="size-5" aria-hidden="true" />
                </span>
                <span className="rounded-full bg-[var(--muted)] px-3 py-1 text-[10px] font-black uppercase tracking-[0.12em] text-[var(--foreground)]">
                  {Math.round(recommendation.confidence * 100)}% confidence
                </span>
              </div>
              <div className="relative mt-5">
                <p className="font-mono text-[10px] font-black uppercase tracking-[0.16em] text-[var(--primary)]">
                  {recommendation.recommendation_type.replace(/_/g, " ")}
                </p>
                <h2 className="mt-2 text-balance font-heading text-xl font-bold text-[var(--foreground)]">
                  {recommendation.title}
                </h2>
                {recommendation.description ? (
                  <p className="mt-3 text-sm leading-6 text-[var(--muted-foreground)]">
                    {recommendation.description}
                  </p>
                ) : null}
                <div className="mt-5 flex items-center gap-2 border-t border-[var(--border)] pt-4 text-xs font-semibold text-[var(--color-forest)]">
                  <ShieldCheck className="size-4" aria-hidden="true" /> Reviewed by a teacher
                </div>
              </div>
            </li>
          </Reveal>
        ))}
      </ul>
    </div>
  );
}
