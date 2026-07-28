"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface AdminTemplateActionResult {
  success?: boolean;
  error?: string;
}

type CreateTemplate = OpenAPI.notification_v1.components["schemas"]["CreateTemplate"];
type UpdateTemplate = OpenAPI.notification_v1.components["schemas"]["UpdateTemplate"];
type Channel = OpenAPI.notification_v1.components["schemas"]["Channel"];
type TemplateStatus = OpenAPI.notification_v1.components["schemas"]["TemplateStatus"];

const CHANNELS: Channel[] = ["email", "sms", "whatsapp", "in_app", "push"];
const STATUSES: TemplateStatus[] = ["active", "archived"];

function value(data: FormData, key: string): string {
  const entry = data.get(key);
  return typeof entry === "string" ? entry.trim() : "";
}

function channel(data: FormData): Channel | null {
  const raw = value(data, "channel");
  return (CHANNELS as string[]).includes(raw) ? (raw as Channel) : null;
}

function revalidateTemplates() {
  revalidatePath("/admin/templates");
  // The journeys builder consumes active templates; keep its catalogue honest too.
  revalidatePath("/admin/journeys");
}

export async function createTemplateAction(
  _previous: AdminTemplateActionResult,
  data: FormData,
): Promise<AdminTemplateActionResult> {
  const name = value(data, "name");
  const selected = channel(data);
  const bodyTemplate = value(data, "body_template");
  if (!name) return { error: "Name the template so staff can recognise it." };
  if (!selected) return { error: "Choose the channel this template sends on." };
  if (!bodyTemplate) return { error: "The body template is required." };
  const subjectTemplate = value(data, "subject_template");
  if (selected === "email" && !subjectTemplate) {
    return { error: "Email templates need a subject line." };
  }
  const body: CreateTemplate = {
    name,
    channel: selected,
    body_template: bodyTemplate,
    ...(subjectTemplate ? { subject_template: subjectTemplate } : {}),
  };
  try {
    const client = await createServerClient();
    await client.post("/api/v1/notification-templates", body);
    revalidateTemplates();
    return { success: true };
  } catch (error) {
    return {
      error: error instanceof Error ? error.message : "Could not create the template.",
    };
  }
}

export async function updateTemplateAction(
  templateId: string,
  _previous: AdminTemplateActionResult,
  data: FormData,
): Promise<AdminTemplateActionResult> {
  const name = value(data, "name");
  const selected = channel(data);
  const bodyTemplate = value(data, "body_template");
  if (!name) return { error: "Name the template so staff can recognise it." };
  if (!selected) return { error: "Choose the channel this template sends on." };
  if (!bodyTemplate) return { error: "The body template is required." };
  const subjectTemplate = value(data, "subject_template");
  if (selected === "email" && !subjectTemplate) {
    return { error: "Email templates need a subject line." };
  }
  const body: UpdateTemplate = {
    name,
    channel: selected,
    body_template: bodyTemplate,
    ...(subjectTemplate ? { subject_template: subjectTemplate } : {}),
  };
  try {
    const client = await createServerClient();
    await client.patch(`/api/v1/notification-templates/${encodeURIComponent(templateId)}`, body);
    revalidateTemplates();
    return { success: true };
  } catch (error) {
    return {
      error: error instanceof Error ? error.message : "Could not update the template.",
    };
  }
}

export async function setTemplateStatusAction(
  templateId: string,
  status: TemplateStatus,
): Promise<AdminTemplateActionResult> {
  if (!(STATUSES as string[]).includes(status)) return { error: "Unknown template status." };
  try {
    const client = await createServerClient();
    await client.patch(`/api/v1/notification-templates/${encodeURIComponent(templateId)}`, {
      status,
    });
    revalidateTemplates();
    return { success: true };
  } catch (error) {
    return {
      error: error instanceof Error ? error.message : "Could not change the template status.",
    };
  }
}

export async function deleteTemplateAction(templateId: string): Promise<AdminTemplateActionResult> {
  try {
    const client = await createServerClient();
    await client.del(`/api/v1/notification-templates/${encodeURIComponent(templateId)}`);
    revalidateTemplates();
    return { success: true };
  } catch (error) {
    return {
      error:
        error instanceof Error
          ? error.message
          : "Could not delete the template. Journeys referencing it may need attention first.",
    };
  }
}
