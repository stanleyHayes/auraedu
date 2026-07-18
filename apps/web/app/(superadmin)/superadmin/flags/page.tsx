import { Flag } from "lucide-react";
import { PageHeader, DataTable, EmptyState, Reveal, Watermark } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient, createServerClientForTenant } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { FEATURE_CATALOG } from "@/lib/feature-catalog";
import { FeatureFlagTenantPicker } from "@/components/feature-flag-tenant-picker";
import { FeatureFlagToggle } from "@/components/feature-flag-toggle";

type Tenant = OpenAPI.tenant_v1.components["schemas"]["Tenant"];
type FeatureFlag = OpenAPI.tenant_v1.components["schemas"]["FeatureFlag"];

interface FlagRow {
  key: string;
  description: string;
  planRequired: string;
  enabled: boolean;
}

interface FeatureFlagsPageProps {
  searchParams: Promise<{ tenant?: string }>;
}

export default async function FeatureFlagsPage({ searchParams }: FeatureFlagsPageProps) {
  await requireAuth();
  const { tenant: tenantParam } = await searchParams;

  let tenants: Tenant[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res = await client.get<{ data?: Tenant[] }>("/api/v1/tenants?limit=100");
    tenants = res.data ?? [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load tenants";
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<Flag className="size-7" />}
          title="Feature flags"
          description="Enable or disable features per school."
        />
        <EmptyState
          title="Could not load tenants"
          description={error}
          icon={<Flag className="size-8" />}
        />
      </div>
    );
  }

  const firstTenant = tenants[0];
  if (!firstTenant) {
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<Flag className="size-7" />}
          title="Feature flags"
          description="Enable or disable features per school."
        />
        <EmptyState
          title="No tenants yet"
          description="Feature flags can be managed once a school is onboarded."
          icon={<Flag className="size-8" />}
        />
      </div>
    );
  }

  const selectedTenant = tenants.find((t) => t.tenant_code === tenantParam) ?? firstTenant;

  // The snapshot endpoint resolves the tenant from the X-Tenant-Code header, so the
  // client must pin the header to the picked tenant (not the host-resolved cookie).
  let flags: FeatureFlag[] = [];
  let flagsError: string | null = null;

  try {
    const client = await createServerClientForTenant(selectedTenant.tenant_code);
    const res = await client.get<{ tenant_code: string; features?: FeatureFlag[] }>(
      "/api/v1/features",
    );
    flags = res.features ?? [];
  } catch (e) {
    flagsError = e instanceof Error ? e.message : "Failed to load feature flags";
  }

  // Merge the snapshot (full catalog with plan_required, joined with persisted state)
  // with the local catalog for descriptions and a stable display order. The API does
  // not expose a state source (default/plan/override) — only the effective on/off state.
  const byKey = new Map(flags.map((f) => [f.feature_key, f]));
  const catalogKeys = new Set(FEATURE_CATALOG.map((entry) => entry.key));

  const rows: FlagRow[] = FEATURE_CATALOG.map((entry) => {
    const flag = byKey.get(entry.key);
    return {
      key: entry.key,
      description: entry.description,
      planRequired: flag?.plan_required ?? entry.planRequired,
      enabled: flag?.is_enabled ?? false,
    };
  });
  for (const flag of flags) {
    if (!catalogKeys.has(flag.feature_key)) {
      rows.push({
        key: flag.feature_key,
        description: "—",
        planRequired: flag.plan_required ?? "—",
        enabled: flag.is_enabled,
      });
    }
  }

  return (
    <div className="relative space-y-6">
      <Watermark className="pointer-events-none absolute -right-6 -top-10 text-[10rem] opacity-[0.03]">
        Flags
      </Watermark>
      <Reveal>
        <PageHeader
          icon={<Flag className="size-7" />}
          title="Feature flags"
          description="Enable or disable features per school. Overrides require a reason and are recorded for audit."
          action={
            <div className="flex items-center gap-3">
              {selectedTenant.plan ? (
                <span className="rounded-full bg-[var(--muted)] px-2.5 py-1 text-xs font-medium capitalize text-[var(--muted-foreground)]">
                  {selectedTenant.plan.replace("_", " ")} plan
                </span>
              ) : null}
              <FeatureFlagTenantPicker
                tenants={tenants.map((t) => ({ tenant_code: t.tenant_code, name: t.name }))}
                selected={selectedTenant.tenant_code}
              />
            </div>
          }
        />
      </Reveal>

      {flagsError ? (
        <EmptyState
          title="Could not load feature flags"
          description={flagsError}
          icon={<Flag className="size-8" />}
        />
      ) : (
        <Reveal delay={80}>
          <DataTable
            caption={`Feature flags for ${selectedTenant.name}`}
            rows={rows}
            keyExtractor={(r) => r.key}
            columns={[
              {
                key: "feature",
                header: "Feature",
                cell: (r) => (
                  <div>
                    <div className="font-mono text-xs">{r.key}</div>
                    <div className="mt-0.5 text-xs text-[var(--muted-foreground)]">
                      {r.description}
                    </div>
                  </div>
                ),
              },
              {
                key: "plan",
                header: "Plan tier",
                className: "w-28",
                cell: (r) => (
                  <span className="rounded-full bg-[var(--muted)] px-2 py-0.5 text-xs capitalize">
                    {r.planRequired.replace("_", " ")}
                  </span>
                ),
              },
              {
                key: "state",
                header: "State",
                className: "w-24",
                cell: (r) =>
                  r.enabled ? (
                    <span className="rounded-full bg-[var(--color-ok)]/10 px-2 py-0.5 text-xs text-[var(--color-ok)]">
                      On
                    </span>
                  ) : (
                    <span className="rounded-full bg-[var(--muted)] px-2 py-0.5 text-xs text-[var(--muted-foreground)]">
                      Off
                    </span>
                  ),
              },
              {
                key: "actions",
                header: "Toggle",
                className: "w-20",
                cell: (r) => (
                  <FeatureFlagToggle
                    tenantCode={selectedTenant.tenant_code}
                    tenantName={selectedTenant.name}
                    featureKey={r.key}
                    enabled={r.enabled}
                  />
                ),
              },
            ]}
            empty={
              <EmptyState
                title="No feature flags"
                description="The feature catalog is empty."
                icon={<Flag className="size-8" />}
              />
            }
          />
        </Reveal>
      )}
    </div>
  );
}
