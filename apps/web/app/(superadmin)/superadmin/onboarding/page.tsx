import { ClipboardList } from "lucide-react";
import {
  PageHeader,
  DataTable,
  EmptyState,
  StatCard,
  Reveal,
  Watermark,
  Button,
  Label,
  Select,
} from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { drilldownQuery, nextPageHref, formatDateTime } from "@/lib/superadmin-drilldown";
import { SuperadminPagination } from "@/components/superadmin-pagination";
import { SuperadminOnboardingDecision } from "@/components/superadmin-onboarding-decision";

type OnboardingRequest = OpenAPI.tenant_v1.components["schemas"]["OnboardingRequest"];

const STATUSES = ["pending_review", "approved", "rejected", "provisioning_failed"] as const;

interface OnboardingPageProps {
  searchParams: Promise<{ status?: string; cursor?: string }>;
}

export default async function OnboardingPage({ searchParams }: OnboardingPageProps) {
  const filters = await searchParams;
  await requireAuth();

  let requests: OnboardingRequest[] = [];
  let nextCursor: string | null = null;
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res = await client.get<{ data?: OnboardingRequest[]; next_cursor?: string | null }>(
      `/api/v1/super-admin/onboarding-requests?${drilldownQuery(
        { status: filters.status },
        filters.cursor,
      )}`,
    );
    requests = res.data ?? [];
    nextCursor = res.next_cursor ?? null;
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load onboarding requests";
  }

  const pending = requests.filter((r) => r.status === "pending_review").length;
  const nextHref = nextPageHref("/superadmin/onboarding", filters, nextCursor);

  return (
    <div className="relative space-y-6">
      <Watermark className="pointer-events-none absolute -right-6 -top-10 text-[10rem] opacity-[0.03]">
        Intake
      </Watermark>
      <Reveal>
        <PageHeader
          icon={<ClipboardList className="size-7" />}
          title="Onboarding requests"
          description="Review school sign-ups, approve provisioning, or reject with a recorded reason."
        />
      </Reveal>

      <Reveal delay={40}>
        <form
          method="get"
          className="card grid gap-3 rounded-[var(--radius-md)] p-4 sm:grid-cols-2 lg:grid-cols-4"
        >
          <div className="space-y-1.5">
            <Label htmlFor="status">Status</Label>
            <Select id="status" name="status" defaultValue={filters.status ?? ""}>
              <option value="">All requests</option>
              {STATUSES.map((status) => (
                <option key={status} value={status}>
                  {status.replace(/_/g, " ")}
                </option>
              ))}
            </Select>
          </div>
          <div className="flex items-end">
            <Button type="submit" variant="secondary" className="w-full">
              Apply filters
            </Button>
          </div>
        </form>
      </Reveal>

      {error ? (
        <EmptyState
          title="Could not load onboarding requests"
          description={error}
          icon={<ClipboardList className="size-8" />}
        />
      ) : (
        <>
          <Reveal delay={60}>
            <section className="grid gap-4 sm:grid-cols-3">
              <StatCard
                label="Requests"
                value={`${requests.length}${nextCursor ? "+" : ""}`}
                unit="this page"
              />
              <StatCard
                label="Pending review"
                value={pending}
                unit="awaiting decision"
                tone={pending > 0 ? "warn" : "default"}
              />
              <StatCard
                label="Decided"
                value={requests.length - pending}
                unit="approved / rejected"
                tone="ok"
              />
            </section>
          </Reveal>

          <Reveal delay={100}>
            <DataTable
              caption="School onboarding requests"
              rows={requests}
              keyExtractor={(r) => r.request_id}
              columns={[
                {
                  key: "school",
                  header: "School",
                  cell: (r) => (
                    <div>
                      <div className="font-medium">{r.school_name}</div>
                      <div className="mt-0.5 text-xs capitalize text-[var(--muted-foreground)]">
                        {r.plan.replace("_", " ")} plan · {r.country_code}
                      </div>
                    </div>
                  ),
                },
                {
                  key: "contact",
                  header: "Contact",
                  cell: (r) => (
                    <div>
                      <div>{r.administrator_name}</div>
                      <div className="mt-0.5 text-xs text-[var(--muted-foreground)]">
                        {r.email}
                        {r.phone ? ` · ${r.phone}` : ""}
                      </div>
                    </div>
                  ),
                },
                {
                  key: "submitted",
                  header: "Submitted",
                  cell: (r) => (
                    <span className="font-mono text-xs">{formatDateTime(r.submitted_at)}</span>
                  ),
                },
                {
                  key: "status",
                  header: "Status",
                  className: "w-36",
                  cell: (r) =>
                    r.status === "approved" ? (
                      <span className="rounded-full bg-[var(--color-ok)]/10 px-2 py-0.5 text-xs capitalize text-[var(--color-ok)]">
                        approved{r.tenant_code ? ` · ${r.tenant_code}` : ""}
                      </span>
                    ) : r.status === "pending_review" ? (
                      <span className="rounded-full bg-[var(--color-warn)]/10 px-2 py-0.5 text-xs capitalize text-[var(--color-warn)]">
                        pending review
                      </span>
                    ) : (
                      <span className="rounded-full bg-[var(--muted)] px-2 py-0.5 text-xs capitalize text-[var(--muted-foreground)]">
                        {r.status.replace(/_/g, " ")}
                      </span>
                    ),
                },
                {
                  key: "actions",
                  header: "Decision",
                  className: "w-24",
                  cell: (r) =>
                    r.status === "pending_review" ? (
                      <SuperadminOnboardingDecision
                        requestId={r.request_id}
                        schoolName={r.school_name}
                      />
                    ) : (
                      <span
                        className="text-xs text-[var(--muted-foreground)]"
                        title={r.decision_reason ?? undefined}
                      >
                        {r.decision_reason ? "Reason recorded" : "—"}
                      </span>
                    ),
                },
              ]}
              empty={
                <EmptyState
                  title="No onboarding requests"
                  description="New school sign-ups will appear here for review."
                  icon={<ClipboardList className="size-8" />}
                />
              }
            />
          </Reveal>

          <SuperadminPagination nextHref={nextHref} />
        </>
      )}
    </div>
  );
}
