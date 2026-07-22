import { PanelsTopLeft } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

export default async function AdminWebsitePage() {
  const client = await createServerClient();
  try {
    const list =
      await client.get<OpenAPI.website_v1.components["schemas"]["PageList"]>(
        "/api/v1/pages?limit=100",
      );
    const rows = list.data ?? [];
    const count = (status: string) => rows.filter((page) => page.status === status).length;
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<PanelsTopLeft className="size-6" />}
          title="School website"
          description="Review public pages, layouts and publication state."
        />
        <section className="grid gap-4 sm:grid-cols-3">
          <StatCard label="Published" value={count("published")} unit="pages" tone="ok" />
          <StatCard label="Draft" value={count("draft")} unit="pages" tone="warn" />
          <StatCard label="Archived" value={count("archived")} unit="pages" />
        </section>
        <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <DataTable
            caption="Website pages"
            rows={rows}
            keyExtractor={(page) => page.id}
            columns={[
              {
                key: "title",
                header: "Page",
                cell: (page) => <span className="font-semibold">{page.title}</span>,
              },
              {
                key: "slug",
                header: "Path",
                cell: (page) => <span className="font-mono text-xs">/{page.slug}</span>,
              },
              {
                key: "layout",
                header: "Layout",
                cell: (page) => <span className="capitalize">{page.layout}</span>,
              },
              {
                key: "status",
                header: "Status",
                cell: (page) => <span className="capitalize">{page.status}</span>,
              },
              {
                key: "updated",
                header: "Updated",
                cell: (page) => new Date(page.updated_at).toLocaleDateString("en-GB"),
              },
            ]}
            empty={
              <EmptyState
                icon={<PanelsTopLeft className="size-8" />}
                title="No website pages"
                description="Pages will appear once the school website is configured."
              />
            }
          />
        </section>
      </div>
    );
  } catch {
    return (
      <EmptyState
        icon={<PanelsTopLeft className="size-8" />}
        title="Website unavailable"
        description="The website content service could not be reached."
      />
    );
  }
}
