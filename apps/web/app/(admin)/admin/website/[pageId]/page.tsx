import Link from "next/link";
import {
  ArrowLeft,
  ChevronDown,
  ChevronUp,
  ExternalLink,
  FileText,
  Image,
  LayoutTemplate,
  Megaphone,
  PanelsTopLeft,
  Phone,
  Star,
  type LucideIcon,
} from "lucide-react";
import { Button, EmptyState, PageHeader } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient, getCurrentTenantCode } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { moveSectionAction } from "@/lib/admin-website-actions";
import { AdminWebsiteSectionSheet } from "@/components/admin-website-section-sheet";
import { AdminWebsiteSectionDeleteButton } from "@/components/admin-website-section-delete-button";

type WebsitePage = OpenAPI.website_v1.components["schemas"]["Page"];
type WebsiteSection = OpenAPI.website_v1.components["schemas"]["Section"];

const TYPE_ICONS: Record<string, LucideIcon> = {
  hero: Star,
  text: FileText,
  features: LayoutTemplate,
  gallery: Image,
  cta: Megaphone,
  contact: Phone,
};

function contentRecord(section: WebsiteSection): Record<string, unknown> {
  const content = section.content;
  return content && typeof content === "object" ? content : {};
}

function summarise(section: WebsiteSection): string {
  const content = contentRecord(section);
  const heading =
    (typeof content.headline === "string" && content.headline) ||
    (typeof content.title === "string" && content.title) ||
    "";
  const body = typeof content.body === "string" ? content.body : "";
  const items = Array.isArray(content.items) ? content.items.length : 0;
  const parts: string[] = [];
  if (heading) parts.push(heading);
  if (body) parts.push(body.length > 80 ? `${body.slice(0, 80)}…` : body);
  if (items > 0) parts.push(`${items} card${items === 1 ? "" : "s"}`);
  return parts.join(" · ") || "No content yet";
}

export default async function AdminWebsiteSectionsPage({
  params,
}: {
  params: Promise<{ pageId: string }>;
}) {
  await requireAuth();
  const { pageId } = await params;
  const [client, tenantCode] = await Promise.all([createServerClient(), getCurrentTenantCode()]);

  let page: WebsitePage | null = null;
  let sections: WebsiteSection[] = [];
  let sectionsError: string | null = null;

  try {
    page = await client.get<WebsitePage>(`/api/v1/website/pages/${encodeURIComponent(pageId)}`);
  } catch {
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<PanelsTopLeft className="size-6" />}
          title="Page not found"
          description="This page could not be loaded from the website service."
        />
        <EmptyState
          icon={<PanelsTopLeft className="size-8" />}
          title="Could not load the page"
          description="It may have been deleted, or the website service is unavailable."
        />
        <Link
          href="/admin/website"
          className="inline-flex items-center gap-2 text-sm font-semibold text-[var(--primary)] hover:underline"
        >
          <ArrowLeft className="size-4" /> Back to pages
        </Link>
      </div>
    );
  }

  try {
    const list = await client.get<OpenAPI.website_v1.components["schemas"]["SectionList"]>(
      `/api/v1/website/pages/${encodeURIComponent(pageId)}/sections?limit=100`,
    );
    sections = (list.data ?? []).slice().sort((a, b) => a.sort_order - b.sort_order);
  } catch (e) {
    sectionsError = e instanceof Error ? e.message : "Failed to load sections";
  }

  const nextSortOrder =
    sections.reduce((max, section) => Math.max(max, section.sort_order), -1) + 1;
  const previewHref =
    tenantCode && page.slug
      ? page.slug === "home"
        ? `/${tenantCode}`
        : `/${tenantCode}/${page.slug}`
      : null;

  return (
    <div className="space-y-6">
      <Link
        href="/admin/website"
        className="inline-flex items-center gap-2 text-sm font-semibold text-muted-foreground hover:text-[var(--primary)]"
      >
        <ArrowLeft className="size-4" /> All pages
      </Link>

      <PageHeader
        icon={<PanelsTopLeft className="size-6" />}
        title={page.title}
        description={`Editing /${page.slug} — sections render top to bottom on the public site.`}
        action={
          <div className="flex flex-wrap items-center gap-2">
            {previewHref ? (
              <a
                href={previewHref}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1.5 rounded-[var(--radius-sm)] border border-border px-3 py-2 text-sm font-semibold text-[var(--primary)] hover:bg-muted"
              >
                <ExternalLink className="size-4" />
                Preview live page
              </a>
            ) : null}
            <AdminWebsiteSectionSheet
              mode="create"
              pageId={page.id}
              nextSortOrder={nextSortOrder}
            />
          </div>
        }
      />

      {sectionsError ? (
        <EmptyState
          icon={<PanelsTopLeft className="size-8" />}
          title="Could not load sections"
          description={sectionsError}
        />
      ) : sections.length === 0 ? (
        <EmptyState
          icon={<PanelsTopLeft className="size-8" />}
          title="No sections yet"
          description="Add a hero banner or text block to start building this page."
        />
      ) : (
        <ol className="space-y-3">
          {sections.map((section, index) => {
            const Icon = TYPE_ICONS[section.type] ?? LayoutTemplate;
            return (
              <li
                key={section.id}
                className="flex flex-wrap items-center gap-4 rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-4 shadow-sm"
              >
                <div className="flex flex-col items-center gap-1">
                  <form
                    action={async () => {
                      "use server";
                      await moveSectionAction(page.id, section.id, "up");
                    }}
                  >
                    <Button
                      type="submit"
                      variant="ghost"
                      className="h-7 px-1.5"
                      disabled={index === 0}
                    >
                      <ChevronUp className="size-4" />
                      <span className="sr-only">Move {section.type} section up</span>
                    </Button>
                  </form>
                  <span className="font-mono text-xs text-muted-foreground">{index + 1}</span>
                  <form
                    action={async () => {
                      "use server";
                      await moveSectionAction(page.id, section.id, "down");
                    }}
                  >
                    <Button
                      type="submit"
                      variant="ghost"
                      className="h-7 px-1.5"
                      disabled={index === sections.length - 1}
                    >
                      <ChevronDown className="size-4" />
                      <span className="sr-only">Move {section.type} section down</span>
                    </Button>
                  </form>
                </div>

                <span className="grid size-11 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
                  <Icon className="size-5" />
                </span>

                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 className="font-heading text-base font-bold capitalize">
                      {section.type.replaceAll("_", " ")}
                    </h2>
                    <span
                      className={`rounded-full px-2 py-0.5 text-xs font-semibold capitalize ${
                        section.status === "published"
                          ? "bg-emerald-50 text-emerald-800"
                          : "bg-amber-50 text-amber-800"
                      }`}
                    >
                      {section.status}
                    </span>
                  </div>
                  <p className="mt-1 truncate text-sm text-muted-foreground">
                    {summarise(section)}
                  </p>
                </div>

                <div className="flex items-center gap-2">
                  <AdminWebsiteSectionSheet
                    mode="edit"
                    pageId={page.id}
                    initial={section}
                    nextSortOrder={nextSortOrder}
                  />
                  <AdminWebsiteSectionDeleteButton
                    pageId={page.id}
                    sectionId={section.id}
                    type={section.type}
                  />
                </div>
              </li>
            );
          })}
        </ol>
      )}
    </div>
  );
}
