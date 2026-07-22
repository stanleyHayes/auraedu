import { Compass, MapPinned } from "lucide-react";
import { PageHeader, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

type Guidance = OpenAPI.career_guidance_v1.components["schemas"]["Guidance"];
type GuardianChildren = OpenAPI.student_v1.components["schemas"]["GuardianChildren"];

interface GuidanceRow {
  guidance: Guidance;
  studentName: string;
}

export default async function ParentGuidancePage() {
  let guidance: GuidanceRow[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const family = await client.get<GuardianChildren>("/api/v1/guardians/me/children");
    const lists = await Promise.all(
      family.students.map(async (student) => {
        const list = await client.get<
          OpenAPI.career_guidance_v1.components["schemas"]["GuidanceList"]
        >(`/api/v1/ai/career-guidance?student_id=${encodeURIComponent(student.id)}`);
        return (list.data ?? []).map((item) => ({
          guidance: item,
          studentName: `${student.first_name} ${student.last_name}`,
        }));
      }),
    );
    guidance = lists.flat();
  } catch {
    error = "Could not load guidance right now.";
  }

  const approved = guidance.filter(({ guidance: item }) => item.status === "approved");

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
          {approved.map(({ guidance: item, studentName }) => (
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
                    {studentName} · {item.guidance_type.replace(/_/g, " ")}
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
