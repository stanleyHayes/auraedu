import Link from "next/link";
import { ExternalLink, LayoutList, PanelsTopLeft } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient, getCurrentTenantCode } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { AdminWebsitePageSheet } from "@/components/admin-website-page-sheet";
import { AdminWebsitePageDeleteButton } from "@/components/admin-website-page-delete-button";

type WebsitePage = OpenAPI.website_v1.components["schemas"]["Page"];

const STATUS_STYLE: Record<string, string> = {
  published: "bg-emerald-50 text-emerald-800",
  draft: "bg-amber-50 text-amber-800",
  archived: "bg-muted text-muted-foreground",
};

function publicPath(tenantCode: string, slug: string) {
  return slug === "home" ? `/${tenantCode}` : `/${tenantCode}/${slug}`;
}

export default async function AdminWebsitePage() {
  await requireAuth();
  const [client, tenantCode] = await Promise.all([createServerClient(), getCurrentTenantCode()]);

  let rows: WebsitePage[] = [];
  try {
    const list = await client.get<OpenAPI.website_v1.components["schemas"]["PageList"]>(
      "/api/v1/website/pages?limit=100",
    );
    rows = list.data ?? [];
  } catch {
    return (
      <EmptyState
        icon={<PanelsTopLeft className="size-8" />}
        title="Website unavailable"
        description="The website content service could not be reached."
      />
    );
  }

  const count = (status: string) => rows.filter((page) => page.status === status).length;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<PanelsTopLeft className="size-6" />}
        title="School website"
        description="Build the public school website: pages, sections and publication state."
        action={
          <div className="flex flex-wrap items-center gap-2">
            {tenantCode ? (
              <a
                href={`/${tenantCode}`}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1.5 rounded-[var(--radius-sm)] border border-border px-3 py-2 text-sm font-semibold text-[var(--primary)] hover:bg-muted"
              >
                <ExternalLink className="size-4" />
                View live site
              </a>
            ) : null}
            <AdminWebsitePageSheet mode="create" />
          </div>
        }
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
              cell: (page) => (
                <span
                  className={`rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${
                    STATUS_STYLE[page.status] ?? "bg-muted text-muted-foreground"
                  }`}
                >
                  {page.status}
                </span>
              ),
            },
            {
              key: "updated",
              header: "Updated",
              cell: (page) => new Date(page.updated_at).toLocaleDateString("en-GB"),
            },
            {
              key: "actions",
              header: "Actions",
              cell: (page) => (
                <div className="flex flex-wrap items-center gap-2">
                  <Link
                    href={`/admin/website/${page.id}`}
                    className="inline-flex items-center gap-1.5 text-sm font-semibold text-[var(--primary)] hover:underline"
                  >
                    <LayoutList className="size-4" />
                    Edit sections
                  </Link>
                  {tenantCode ? (
                    <a
                      href={publicPath(tenantCode, page.slug)}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-[var(--primary)]"
                    >
                      <ExternalLink className="size-3.5" />
                      Preview
                    </a>
                  ) : null}
                  <AdminWebsitePageSheet mode="edit" initial={page} />
                  <AdminWebsitePageDeleteButton id={page.id} title={page.title} />
                </div>
              ),
            },
          ]}
          empty={
            <EmptyState
              icon={<PanelsTopLeft className="size-8" />}
              title="No website pages"
              description='Create your first page — the homepage uses the slug "home".'
            />
          }
        />
      </section>
    </div>
  );
}
