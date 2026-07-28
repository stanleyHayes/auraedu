"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

type Plan = OpenAPI.billing_v1.components["schemas"]["Plan"];
type CreatePlan = OpenAPI.billing_v1.components["schemas"]["CreatePlan"];
type UpdatePlan = OpenAPI.billing_v1.components["schemas"]["UpdatePlan"];
type Subscription = OpenAPI.billing_v1.components["schemas"]["Subscription"];

export interface BillingActionResult {
  success?: boolean;
  error?: string;
}

const SUBSCRIPTION_STATUSES = ["trialing", "active", "past_due", "cancelled"] as const;

function parsePlanForm(formData: FormData): { body?: CreatePlan; error?: string } {
  const code = String((formData.get("code") as string | null) ?? "").trim();
  const name = String((formData.get("name") as string | null) ?? "").trim();
  if (!code || !name) {
    return { error: "Plan code and name are required." };
  }

  const priceMajor = Number((formData.get("price") as string | null) ?? "");
  if (!Number.isFinite(priceMajor) || priceMajor < 0) {
    return { error: "Enter a valid price (0 or more)." };
  }

  const currency = String((formData.get("currency") as string | null) ?? "")
    .trim()
    .toUpperCase();
  if (!/^[A-Z]{3}$/.test(currency)) {
    return { error: "Currency must be a 3-letter ISO code (e.g. GHS)." };
  }

  const billingInterval = formData.get("billing_interval") === "yearly" ? "yearly" : "monthly";
  const description = String((formData.get("description") as string | null) ?? "").trim() || null;
  const features = String((formData.get("features") as string | null) ?? "")
    .split(/[\n,]/)
    .map((f) => f.trim())
    .filter(Boolean);

  return {
    body: {
      code,
      name,
      description,
      price_cents: Math.round(priceMajor * 100),
      currency,
      billing_interval: billingInterval,
      features,
    },
  };
}

/** Create a SaaS plan (POST /api/v1/billing/plans, contract billing.v1). */
export async function createPlanAction(
  _prev: BillingActionResult,
  formData: FormData,
): Promise<BillingActionResult> {
  const { body, error } = parsePlanForm(formData);
  if (!body) return { error };

  const client = await createServerClient();
  try {
    await client.post<Plan>("/api/v1/billing/plans", body);
    revalidatePath("/superadmin/billing-plans");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to create plan." };
  }
}

/** Update a SaaS plan (PATCH /api/v1/billing/plans/{plan_id}, contract billing.v1). */
export async function updatePlanAction(
  planId: string,
  _prev: BillingActionResult,
  formData: FormData,
): Promise<BillingActionResult> {
  const { body, error } = parsePlanForm(formData);
  if (!body) return { error };

  const status = (formData.get("status") as string | null) ?? undefined;
  const update: UpdatePlan = {
    ...body,
    ...(status === "active" || status === "archived" ? { status } : {}),
  };

  const client = await createServerClient();
  try {
    await client.patch<Plan>(`/api/v1/billing/plans/${encodeURIComponent(planId)}`, update);
    revalidatePath("/superadmin/billing-plans");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to update plan." };
  }
}

/** Delete a SaaS plan (DELETE /api/v1/billing/plans/{plan_id}, contract billing.v1). */
export async function deletePlanAction(planId: string): Promise<BillingActionResult> {
  const client = await createServerClient();
  try {
    await client.del(`/api/v1/billing/plans/${encodeURIComponent(planId)}`);
    revalidatePath("/superadmin/billing-plans");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to delete plan." };
  }
}

/**
 * Move a subscription to another plan
 * (POST /api/v1/billing/subscriptions/{subscription_id}/change-plan, contract billing.v1).
 */
export async function changeSubscriptionPlanAction(
  subscriptionId: string,
  _prev: BillingActionResult,
  formData: FormData,
): Promise<BillingActionResult> {
  const planId = String((formData.get("plan_id") as string | null) ?? "").trim();
  if (!planId) {
    return { error: "Choose the plan to move this subscription to." };
  }

  const client = await createServerClient();
  try {
    await client.post<Subscription>(
      `/api/v1/billing/subscriptions/${encodeURIComponent(subscriptionId)}/change-plan`,
      { plan_id: planId },
    );
    revalidatePath("/superadmin/subscriptions");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to change the subscription plan." };
  }
}

/**
 * Change a subscription's lifecycle status
 * (PATCH /api/v1/billing/subscriptions/{subscription_id}, contract billing.v1).
 */
export async function updateSubscriptionStatusAction(
  subscriptionId: string,
  _prev: BillingActionResult,
  formData: FormData,
): Promise<BillingActionResult> {
  const status = String((formData.get("status") as string | null) ?? "").trim();
  if (!SUBSCRIPTION_STATUSES.includes(status as (typeof SUBSCRIPTION_STATUSES)[number])) {
    return { error: "Choose a valid subscription status." };
  }

  const client = await createServerClient();
  try {
    await client.patch<Subscription>(
      `/api/v1/billing/subscriptions/${encodeURIComponent(subscriptionId)}`,
      { status: status as Subscription["status"] },
    );
    revalidatePath("/superadmin/subscriptions");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to update the subscription." };
  }
}
