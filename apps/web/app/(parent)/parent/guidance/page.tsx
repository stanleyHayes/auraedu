import { Compass, MapPinned } from "lucide-react";
import { PageHeader, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient, getSession } from "@/lib/api";

type Guidance = OpenAPI.career_guidance_v1.components["schemas"]["Guidance"];

export default async function ParentGuidancePage() {
  const session = await getSession();
  let guidance: Guidance[] = [];
  let error: string | null = null;

  // student_id is required by the contract; without a session there is nothing
  // to query, so fall through to the empty state.
  if (session?.user_id) {
    try {
      const client = await createServerClient();
      // NOTE: student_id is the identity user id until the backend exposes an
      // actor→student-record mapping for guardians' children.
      const list = await client.get<
        OpenAPI.career_guidance_v1.components["schemas"]["GuidanceList"]
      >(`/api/v1/ai/career-guidance?student_id=${session.user_id}`);
      guidance = list.data ?? [];
    } catch {
      error = "Could not load guidance right now.";
    }
  }

  // The list endpoint has no server-side status filter, so only approved
  // guidance is shown here.
  const approved = guidance.filter((g) => g.status === "approved");

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Compass className="size-6" />}
        title="Guidance"
        description="Career guidance and counselling updates for your children."
      />
      {error ? (
        <EmptyState
          icon={<MapPinned className="size-8" />}
          title="Guidance unavailable"
          description={error}
        />
      ) : approved.length === 0 ? (
        <EmptyState
          icon={<MapPinned className="size-8" />}
          title="No guidance updates"
          description="Career guidance notes and recommendations will appear here."
        />
      ) : (
        <ul className="space-y-3">
          {approved.map((item) => (
            <li
              key={item.id}
              className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-4"
            >
              <div className="flex items-start justify-between gap-4">
                <div>
                  <h3 className="font-medium text-[var(--foreground)]">{item.title}</h3>
                  {item.explanation ? (
                    <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                      {item.explanation}
                    </p>
                  ) : null}
                  <p className="mt-1 text-xs capitalize text-[var(--muted-foreground)]">
                    {item.guidance_type.replace(/_/g, " ")}
                  </p>
                </div>
                <span className="shrink-0 text-xs text-[var(--muted-foreground)]">
                  {Math.round(item.confidence * 100)}% confidence
                </span>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
