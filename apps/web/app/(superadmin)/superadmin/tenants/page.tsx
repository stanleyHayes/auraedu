import Link from "next/link";
import { Building2, Pencil } from "lucide-react";
import { PageHeader, DataTable, EmptyState, Button, Reveal, Watermark } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { TenantCreateSheet } from "@/components/tenant-create-sheet";
import { DeleteTenantButton } from "@/components/delete-tenant-button";

export default async function TenantsPage() {
  await requireAuth();

  let tenants: OpenAPI.tenant_v1.components["schemas"]["Tenant"][] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res = await client.get<{ data?: OpenAPI.tenant_v1.components["schemas"]["Tenant"][] }>(
      "/api/v1/tenants?limit=50",
    );
    tenants = res.data ?? [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load tenants";
  }

  return (
    <div className="relative space-y-6">
      <Watermark className="pointer-events-none absolute -right-6 -top-10 text-[10rem] opacity-[0.03]">
        Tenants
      </Watermark>
      <Reveal>
        <PageHeader
          icon={<Building2 className="size-7" />}
          title="Tenants"
          description="View all schools and organisations on the platform."
          action={<TenantCreateSheet />}
        />
      </Reveal>

      {error ? (
        <EmptyState
          title="Could not load tenants"
          description={error}
          icon={<Building2 className="size-8" />}
        />
      ) : (
        <Reveal delay={80}>
          <DataTable
            caption="Tenants"
            rows={tenants}
            keyExtractor={(t) => t.tenant_code}
            columns={[
              {
                key: "code",
                header: "Code",
                cell: (t) => <span className="font-mono text-xs">{t.tenant_code}</span>,
              },
              {
                key: "name",
                header: "Name",
                cell: (t) => t.name,
              },
              {
                key: "status",
                header: "Status",
                cell: (t) => <span className="capitalize">{t.status}</span>,
              },
              {
                key: "plan",
                header: "Plan",
                cell: (t) => <span className="capitalize">{t.plan ?? "—"}</span>,
              },
              {
                key: "domain",
                header: "Domain",
                cell: (t) => t.domain ?? "—",
              },
              {
                key: "actions",
                header: "Actions",
                className: "w-24",
                cell: (t) => (
                  <div className="flex items-center gap-2">
                    <Button asChild variant="ghost" className="h-8 px-2">
                      <Link href={`/superadmin/tenants/${t.tenant_code}`}>
                        <Pencil className="size-4" />
                        <span className="sr-only">Edit {t.name}</span>
                      </Link>
                    </Button>
                    <DeleteTenantButton code={t.tenant_code} name={t.name} />
                  </div>
                ),
              },
            ]}
            empty={
              <EmptyState
                title="No tenants yet"
                description="Tenants will appear here once schools are onboarded."
                icon={<Building2 className="size-8" />}
              />
            }
          />
        </Reveal>
      )}
    </div>
  );
}
