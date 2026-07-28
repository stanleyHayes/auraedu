"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { SlidersHorizontal } from "lucide-react";
import { Button, Label, Select, Sheet } from "@auraedu/ui";
import {
  changeSubscriptionPlanAction,
  updateSubscriptionStatusAction,
  type BillingActionResult,
} from "@/lib/superadmin-billing-actions";

const STATUSES = ["trialing", "active", "past_due", "cancelled"] as const;

export interface SuperadminSubscriptionActionsProps {
  subscriptionId: string;
  tenantLabel: string;
  currentPlanId: string;
  currentStatus: string;
  plans: { id: string; name: string; code: string }[];
}

/**
 * Manage sheet for one subscription: move it to another plan
 * (POST .../change-plan) or change its lifecycle status (PATCH), contract billing.v1.
 */
export function SuperadminSubscriptionActions({
  subscriptionId,
  tenantLabel,
  currentPlanId,
  currentStatus,
  plans,
}: SuperadminSubscriptionActionsProps) {
  const router = useRouter();
  const [open, setOpen] = React.useState(false);

  const [planState, planFormAction, planPending] = React.useActionState<
    BillingActionResult,
    FormData
  >(changeSubscriptionPlanAction.bind(null, subscriptionId), {});
  const [statusState, statusFormAction, statusPending] = React.useActionState<
    BillingActionResult,
    FormData
  >(updateSubscriptionStatusAction.bind(null, subscriptionId), {});

  React.useEffect(() => {
    if (planState.success || statusState.success) {
      router.refresh();
    }
  }, [planState, statusState, router]);

  return (
    <>
      <Button type="button" variant="ghost" className="h-8 px-2" onClick={() => setOpen(true)}>
        <SlidersHorizontal className="size-4" />
        <span className="sr-only">Manage subscription for {tenantLabel}</span>
      </Button>

      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-md bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <div className="border-b border-[var(--border)] bg-[var(--muted)] px-6 py-4">
            <h2 className="font-heading text-lg font-bold">Manage subscription</h2>
            <p className="text-sm text-muted-foreground">
              Change the plan or lifecycle status for tenant{" "}
              <span className="font-mono text-xs">{tenantLabel}</span>.
            </p>
          </div>
          <div className="flex-1 space-y-8 overflow-y-auto p-6">
            <form action={planFormAction} className="space-y-4">
              <h3 className="font-sans font-semibold tracking-tight">Change plan</h3>
              <div className="space-y-1.5">
                <Label htmlFor={`plan-${subscriptionId}`}>New plan</Label>
                <Select
                  id={`plan-${subscriptionId}`}
                  name="plan_id"
                  defaultValue={currentPlanId}
                  required
                >
                  {plans.map((plan) => (
                    <option key={plan.id} value={plan.id}>
                      {plan.name} ({plan.code})
                    </option>
                  ))}
                </Select>
              </div>
              {planState.error ? (
                <p className="text-sm text-destructive">{planState.error}</p>
              ) : null}
              {planState.success ? (
                <p className="text-sm text-[var(--color-ok)]">Plan updated.</p>
              ) : null}
              <Button type="submit" loading={planPending} loadingLabel="Changing">
                Change plan
              </Button>
            </form>

            <form
              action={statusFormAction}
              className="space-y-4 border-t border-[var(--border)] pt-6"
            >
              <h3 className="font-sans font-semibold tracking-tight">Change status</h3>
              <div className="space-y-1.5">
                <Label htmlFor={`status-${subscriptionId}`}>New status</Label>
                <Select
                  id={`status-${subscriptionId}`}
                  name="status"
                  defaultValue={currentStatus}
                  required
                >
                  {STATUSES.map((status) => (
                    <option key={status} value={status}>
                      {status.replace("_", " ")}
                    </option>
                  ))}
                </Select>
              </div>
              {statusState.error ? (
                <p className="text-sm text-destructive">{statusState.error}</p>
              ) : null}
              {statusState.success ? (
                <p className="text-sm text-[var(--color-ok)]">Status updated.</p>
              ) : null}
              <Button
                type="submit"
                variant="secondary"
                loading={statusPending}
                loadingLabel="Updating"
              >
                Update status
              </Button>
            </form>
          </div>
        </div>
      </Sheet>
    </>
  );
}
