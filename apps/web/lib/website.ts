import { gatewayInternalUrl, tenantHeaderName } from "@auraedu/config";

export type WebsiteSectionContent = Record<string, unknown> | string | null;

export interface FeatureItem {
  title?: string;
  description?: string;
  icon?: string;
}

export interface WebsiteSection {
  id: string;
  tenant_id: string;
  page_id: string;
  type: "hero" | "text" | "features" | "gallery" | "cta" | "contact" | "call_to_action";
  content: WebsiteSectionContent;
  sort_order: number;
  status: string;
}

export interface WebsitePage {
  id: string;
  tenant_id: string;
  slug: string;
  title: string;
  status: string;
  meta_description?: string;
  layout: string;
  sections?: WebsiteSection[];
}

interface WebsitePageList {
  data: WebsitePage[];
  next_cursor?: string | null;
}

interface WebsiteSectionList {
  data: WebsiteSection[];
  next_cursor?: string | null;
}

function tenantHeaders(tenantCode: string): HeadersInit {
  return { [tenantHeaderName]: tenantCode };
}

function normalizeContent(content: WebsiteSectionContent): Record<string, unknown> {
  if (content && typeof content === "object") return content;
  if (typeof content === "string") return { body: content };
  return {};
}

export function getContentValue(content: WebsiteSectionContent): Record<string, unknown> {
  return normalizeContent(content);
}

export async function fetchWebsitePages(
  tenantCode: string,
  { status }: { status?: string } = {},
): Promise<WebsitePage[]> {
  const url = new URL("/api/v1/website/pages", gatewayInternalUrl);
  if (status) url.searchParams.set("status", status);

  try {
    const res = await fetch(url, {
      headers: tenantHeaders(tenantCode),
      next: { revalidate: 60 },
    });
    if (!res.ok) return [];
    const json = (await res.json()) as WebsitePageList;
    return json.data ?? [];
  } catch {
    return [];
  }
}

export async function fetchPageBySlug(
  tenantCode: string,
  slug: string,
): Promise<WebsitePage | null> {
  const url = new URL(`/api/v1/website/page-slugs/${encodeURIComponent(slug)}`, gatewayInternalUrl);

  try {
    const res = await fetch(url, {
      headers: tenantHeaders(tenantCode),
      next: { revalidate: 60 },
    });
    if (!res.ok) return null;
    const page = (await res.json()) as WebsitePage;
    const sections = await fetchSections(tenantCode, page.id);
    return { ...page, sections };
  } catch {
    return null;
  }
}

async function fetchSections(tenantCode: string, pageId: string): Promise<WebsiteSection[]> {
  const url = new URL(
    `/api/v1/website/pages/${encodeURIComponent(pageId)}/sections`,
    gatewayInternalUrl,
  );

  try {
    const res = await fetch(url, {
      headers: tenantHeaders(tenantCode),
      next: { revalidate: 60 },
    });
    if (!res.ok) return [];
    const json = (await res.json()) as WebsiteSectionList;
    return (json.data ?? [])
      .filter((section) => section.status === "published")
      .sort((a, b) => a.sort_order - b.sort_order);
  } catch {
    return [];
  }
}
