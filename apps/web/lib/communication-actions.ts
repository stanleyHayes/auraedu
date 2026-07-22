"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface CommunicationActionResult {
  success?: boolean;
  error?: string;
}
type CreateAnnouncement = OpenAPI.notification_v1.components["schemas"]["CreateAnnouncement"];

function value(data: FormData, key: string): string {
  const entry = data.get(key);
  return typeof entry === "string" ? entry.trim() : "";
}

export async function createAnnouncementAction(
  _previous: CommunicationActionResult,
  data: FormData,
): Promise<CommunicationActionResult> {
  const title = value(data, "title");
  const bodyText = value(data, "body");
  if (title.length < 3) return { error: "Use a clear title of at least three characters." };
  if (bodyText.length < 3) return { error: "Announcement details are required." };
  const body: CreateAnnouncement = {
    title,
    body: bodyText,
    audience: value(data, "audience") as CreateAnnouncement["audience"],
  };
  try {
    const client = await createServerClient();
    await client.post("/api/v1/announcements", body);
    revalidatePath("/admin/communications");
    return { success: true };
  } catch (error) {
    return {
      error: error instanceof Error ? error.message : "Could not publish the announcement.",
    };
  }
}

export async function deleteAnnouncementAction(id: string): Promise<CommunicationActionResult> {
  try {
    const client = await createServerClient();
    await client.del(`/api/v1/announcements/${encodeURIComponent(id)}`);
    revalidatePath("/admin/communications");
    return { success: true };
  } catch (error) {
    return { error: error instanceof Error ? error.message : "Could not remove the announcement." };
  }
}
