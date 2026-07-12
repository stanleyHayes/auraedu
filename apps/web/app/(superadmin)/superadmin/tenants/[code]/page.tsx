import { notFound } from "next/navigation";
import { Building2, Settings2 } from "lucide-react";
import { PageHeader, Reveal, Watermark } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { TenantForm } from "@/components/tenant-form";
import { TenantSettingsForm } from "@/components/tenant-settings-form";

interface TenantDetailPageProps {
  params: Promise<{ code: string }>;
}

export default async function TenantDetailPage({ params }: TenantDetailPageProps) {
  const { code } = await params;
  await requireAuth();

  const client = await createServerClient();

  let tenant: OpenAPI.tenant_v1.components["schemas"]["Tenant"] | null = null;
  let settings: OpenAPI.tenant_v1.components["schemas"]["Settings"] | undefined;
  let error: string | null = null;

  try {
    [tenant, settings] = await Promise.all([
      client.get<OpenAPI.tenant_v1.components["schemas"]["Tenant"]>(
        `/api/v1/tenants/${encodeURIComponent(code)}`,
      ),
      client
        .get<OpenAPI.tenant_v1.components["schemas"]["Settings"]>(
          `/api/v1/tenants/${encodeURIComponent(code)}/settings`,
        )
        .catch(() => undefined),
    ]);
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load tenant";
  }

  if (!tenant) {
    if (error) {
      return (
        <div className="space-y-6">
          <PageHeader
            icon={<Building2 className="size-7" />}
            title="Tenant"
            description="Could not load this tenant."
          />
          <p className="text-sm text-destructive">{error}</p>
        </div>
      );
    }
    notFound();
  }

  return (
    <div className="relative space-y-8">
      <Watermark className="pointer-events-none absolute -right-6 -top-10 text-[10rem] opacity-[0.03]">
        {tenant.short}
      </Watermark>
      <Reveal>
        <PageHeader
          icon={<Building2 className="size-7" />}
          title={tenant.name}
          description={`Tenant code: ${tenant.tenant_code}`}
        />
      </Reveal>

      <Reveal delay={80}>
        <section className="card rounded-[var(--radius-lg)] p-6">
          <h2 className="mb-1 font-heading text-lg font-bold">Profile & branding</h2>
          <p className="mb-6 text-sm text-muted-foreground">
            Core identity, domain, plan, and logo for this school.
          </p>
          <TenantForm mode="edit" tenantCode={code} initial={tenant} />
        </section>
      </Reveal>

      <Reveal delay={120}>
        <section className="card rounded-[var(--radius-lg)] p-6">
          <div className="mb-6 flex items-start gap-3">
            <Settings2 className="mt-0.5 size-5 text-[var(--primary)]" />
            <div>
              <h2 className="font-heading text-lg font-bold">Operational settings</h2>
              <p className="text-sm text-muted-foreground">
                Locale, timezone, date format, and academic-year settings.
              </p>
            </div>
          </div>
          <TenantSettingsForm tenantCode={code} initial={settings} />
        </section>
      </Reveal>
    </div>
  );
}
