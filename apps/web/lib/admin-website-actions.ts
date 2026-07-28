"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface AdminWebsiteActionResult {
  success?: boolean;
  error?: string;
}

type CreatePage = OpenAPI.website_v1.components["schemas"]["CreatePage"];
type UpdatePage = OpenAPI.website_v1.components["schemas"]["UpdatePage"];
type CreateSection = OpenAPI.website_v1.components["schemas"]["CreateSection"];
type UpdateSection = OpenAPI.website_v1.components["schemas"]["UpdateSection"];

const SECTION_TYPES = ["hero", "text", "features", "gallery", "cta", "contact"] as const;

function field(formData: FormData, key: string): string {
  return String((formData.get(key) as string | null) ?? "").trim();
}

function texts(formData: FormData, key: string): string[] {
  return formData.getAll(key).map((value) => (typeof value === "string" ? value.trim() : ""));
}

/**
 * Assemble the free-form section content object from the type-specific fields,
 * matching the keys the public site renderer (`components/website-section.tsx`) reads.
 */
function buildSectionContent(formData: FormData): Record<string, unknown> {
  const type = field(formData, "type");
  const content: Record<string, unknown> = {};
  const put = (key: string) => {
    const value = field(formData, key);
    if (value) content[key] = value;
  };

  switch (type) {
    case "hero":
      put("headline");
      put("body");
      put("cta_label");
      put("cta_url");
      break;
    case "text":
      put("title");
      put("body");
      break;
    case "features":
    case "gallery": {
      put("title");
      put("body");
      const titles = texts(formData, "item_title");
      const descriptions = texts(formData, "item_description");
      const icons = texts(formData, "item_icon");
      const items = titles
        .map((title, index) => ({
          title,
          description: descriptions[index] ?? undefined,
          icon: icons[index] ?? undefined,
        }))
        .filter((item) => item.title);
      if (items.length > 0) content.items = items;
      break;
    }
    case "cta":
      put("title");
      put("body");
      put("cta_label");
      put("cta_url");
      break;
    case "contact":
      put("title");
      put("email");
      put("phone");
      put("address");
      break;
    default:
      break;
  }
  return content;
}

export async function createPageAction(
  _prev: AdminWebsiteActionResult,
  formData: FormData,
): Promise<AdminWebsiteActionResult> {
  const slug = field(formData, "slug")
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/^-+|-+$/g, "");
  const title = field(formData, "title");
  if (!slug) return { error: "A URL slug is required (letters, numbers and hyphens)." };
  if (!title) return { error: "Page title is required." };

  const body: CreatePage = {
    slug,
    title,
    status: (field(formData, "status") || "draft") as CreatePage["status"],
    layout: (field(formData, "layout") || "default") as CreatePage["layout"],
    meta_description: field(formData, "meta_description") || null,
  };
  try {
    const client = await createServerClient();
    await client.post("/api/v1/website/pages", body);
    revalidatePath("/admin/website");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to create the page." };
  }
}

export async function updatePageAction(
  id: string,
  _prev: AdminWebsiteActionResult,
  formData: FormData,
): Promise<AdminWebsiteActionResult> {
  const slug = field(formData, "slug")
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/^-+|-+$/g, "");
  const title = field(formData, "title");
  if (!slug) return { error: "A URL slug is required (letters, numbers and hyphens)." };
  if (!title) return { error: "Page title is required." };

  const body: UpdatePage = {
    slug,
    title,
    status: field(formData, "status") as UpdatePage["status"],
    layout: field(formData, "layout") as UpdatePage["layout"],
    meta_description: field(formData, "meta_description") || null,
  };
  try {
    const client = await createServerClient();
    await client.patch(`/api/v1/website/pages/${encodeURIComponent(id)}`, body);
    revalidatePath("/admin/website");
    revalidatePath(`/admin/website/${id}`);
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to update the page." };
  }
}

export async function deletePageAction(id: string): Promise<AdminWebsiteActionResult> {
  try {
    const client = await createServerClient();
    await client.del(`/api/v1/website/pages/${encodeURIComponent(id)}`);
    revalidatePath("/admin/website");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to delete the page." };
  }
}

export async function createSectionAction(
  pageId: string,
  _prev: AdminWebsiteActionResult,
  formData: FormData,
): Promise<AdminWebsiteActionResult> {
  const type = field(formData, "type");
  if (!(SECTION_TYPES as readonly string[]).includes(type)) {
    return { error: "Choose a section type." };
  }
  const sortOrder = Number(field(formData, "sort_order"));
  const body: CreateSection = {
    type: type as CreateSection["type"],
    content: buildSectionContent(formData),
    sort_order: Number.isInteger(sortOrder) && sortOrder >= 0 ? sortOrder : 0,
    status: (field(formData, "status") || "draft") as CreateSection["status"],
  };
  try {
    const client = await createServerClient();
    await client.post(`/api/v1/website/pages/${encodeURIComponent(pageId)}/sections`, body);
    revalidatePath(`/admin/website/${pageId}`);
    revalidatePath("/admin/website");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to create the section." };
  }
}

export async function updateSectionAction(
  pageId: string,
  sectionId: string,
  _prev: AdminWebsiteActionResult,
  formData: FormData,
): Promise<AdminWebsiteActionResult> {
  const type = field(formData, "type");
  if (!(SECTION_TYPES as readonly string[]).includes(type)) {
    return { error: "Choose a section type." };
  }
  const body: UpdateSection = {
    type: type as UpdateSection["type"],
    content: buildSectionContent(formData),
    status: field(formData, "status") as UpdateSection["status"],
  };
  try {
    const client = await createServerClient();
    await client.patch(`/api/v1/website/sections/${encodeURIComponent(sectionId)}`, body);
    revalidatePath(`/admin/website/${pageId}`);
    revalidatePath("/admin/website");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to update the section." };
  }
}

export async function deleteSectionAction(
  pageId: string,
  sectionId: string,
): Promise<AdminWebsiteActionResult> {
  try {
    const client = await createServerClient();
    await client.del(`/api/v1/website/sections/${encodeURIComponent(sectionId)}`);
    revalidatePath(`/admin/website/${pageId}`);
    revalidatePath("/admin/website");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to delete the section." };
  }
}

type SectionList = OpenAPI.website_v1.components["schemas"]["SectionList"];

/**
 * Swap a section with its neighbour and normalise sort_order across the page so
 * ordering stays deterministic even when stored values collide.
 */
export async function moveSectionAction(
  pageId: string,
  sectionId: string,
  direction: "up" | "down",
): Promise<AdminWebsiteActionResult> {
  try {
    const client = await createServerClient();
    const list = await client.get<SectionList>(
      `/api/v1/website/pages/${encodeURIComponent(pageId)}/sections?limit=100`,
    );
    const sections = (list.data ?? []).slice().sort((a, b) => a.sort_order - b.sort_order);
    const index = sections.findIndex((section) => section.id === sectionId);
    if (index === -1) return { error: "Section not found on this page." };
    const target = direction === "up" ? index - 1 : index + 1;
    if (target < 0 || target >= sections.length) return { success: true };

    const reordered = [...sections];
    const moved = reordered[index];
    if (!moved) return { error: "Section not found on this page." };
    reordered.splice(index, 1);
    reordered.splice(target, 0, moved);
    await Promise.all(
      reordered.map((section, order) =>
        section.sort_order === order
          ? Promise.resolve()
          : client.patch(`/api/v1/website/sections/${encodeURIComponent(section.id)}`, {
              sort_order: order,
            }),
      ),
    );
    revalidatePath(`/admin/website/${pageId}`);
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to reorder the section." };
  }
}
