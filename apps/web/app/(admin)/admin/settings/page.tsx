import { Settings2 } from "lucide-react";
import { EmptyState, PageHeader, Reveal } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { CustomDomainCard } from "@/components/custom-domain-card";
import { TenantSettingsForm } from "@/components/tenant-settings-form";
import { createServerClient, getCurrentTenantCode } from "@/lib/api";

export default async function AdminSettingsPage() {
  const tenantCode = await getCurrentTenantCode();
  if (!tenantCode) {
    return (
      <EmptyState
        icon={<Settings2 className="size-8" />}
        title="School context unavailable"
        description="A resolved tenant is required to manage school settings."
      />
    );
  }
  const client = await createServerClient();
  let settings: OpenAPI.tenant_v1.components["schemas"]["Settings"];
  let customDomain: OpenAPI.tenant_v1.components["schemas"]["CustomDomain"] | undefined;
  try {
    [settings, customDomain] = await Promise.all([
      client.get<OpenAPI.tenant_v1.components["schemas"]["Settings"]>(
        `/api/v1/tenants/${encodeURIComponent(tenantCode)}/settings`,
      ),
      client
        .get<OpenAPI.tenant_v1.components["schemas"]["CustomDomain"]>(
          `/api/v1/tenants/${encodeURIComponent(tenantCode)}/custom-domain`,
        )
        .catch(() => undefined),
    ]);
  } catch {
    return (
      <EmptyState
        icon={<Settings2 className="size-8" />}
        title="Settings unavailable"
        description="School settings could not be loaded."
      />
    );
  }
  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Settings2 className="size-6" />}
        title="School settings"
        description="Manage operations, identity, and the school’s verified public address."
      />
      <Reveal>
        <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-6">
          <TenantSettingsForm tenantCode={tenantCode} initial={settings} />
        </section>
      </Reveal>
      <Reveal delay={80}>
        <CustomDomainCard tenantCode={tenantCode} initial={customDomain} />
      </Reveal>
    </div>
  );
}
