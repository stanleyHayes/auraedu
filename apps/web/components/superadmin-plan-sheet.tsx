"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Pencil, Plus } from "lucide-react";
import { Button, Input, Label, Select, Sheet } from "@auraedu/ui";
import {
  createPlanAction,
  updatePlanAction,
  type BillingActionResult,
} from "@/lib/superadmin-billing-actions";

export interface PlanInitial {
  id: string;
  code: string;
  name: string;
  description?: string | null;
  price_cents: number;
  currency: string;
  billing_interval: string;
  features?: string[];
  status?: string;
}

export interface SuperadminPlanSheetProps {
  mode: "create" | "edit";
  plan?: PlanInitial;
}

/**
 * Create / edit sheet for a SaaS plan. Price is captured in major currency units and
 * converted to cents by the server action (contract billing.v1 CreatePlan/UpdatePlan).
 */
export function SuperadminPlanSheet({ mode, plan }: SuperadminPlanSheetProps) {
  const router = useRouter();
  const [open, setOpen] = React.useState(false);

  const action = mode === "create" ? createPlanAction : updatePlanAction.bind(null, plan?.id ?? "");
  const [state, formAction, pending] = React.useActionState<BillingActionResult, FormData>(
    action,
    {},
  );

  React.useEffect(() => {
    if (state.success) {
      setOpen(false);
      router.refresh();
    }
  }, [state, router]);

  return (
    <>
      {mode === "create" ? (
        <Button type="button" onClick={() => setOpen(true)}>
          <Plus className="mr-2 size-4" />
          Add plan
        </Button>
      ) : (
        <Button type="button" variant="ghost" className="h-8 px-2" onClick={() => setOpen(true)}>
          <Pencil className="size-4" />
          <span className="sr-only">Edit {plan?.name}</span>
        </Button>
      )}

      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-xl bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <div className="border-b border-[var(--border)] bg-[var(--muted)] px-6 py-4">
            <h2 className="font-heading text-lg font-bold">
              {mode === "create" ? "Add plan" : `Edit ${plan?.name ?? "plan"}`}
            </h2>
            <p className="text-sm text-muted-foreground">
              {mode === "create"
                ? "Create a SaaS plan schools can subscribe to."
                : "Update pricing, features, or archive this plan."}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <form action={formAction} className="space-y-5">
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-1.5">
                  <Label htmlFor="plan-code">Code (required)</Label>
                  <Input
                    id="plan-code"
                    name="code"
                    required
                    defaultValue={plan?.code ?? ""}
                    placeholder="e.g. growth"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="plan-name">Name (required)</Label>
                  <Input
                    id="plan-name"
                    name="name"
                    required
                    defaultValue={plan?.name ?? ""}
                    placeholder="e.g. Growth"
                  />
                </div>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="plan-description">Description</Label>
                <Input
                  id="plan-description"
                  name="description"
                  defaultValue={plan?.description ?? ""}
                  placeholder="Who this plan is for"
                />
              </div>

              <div className="grid gap-4 sm:grid-cols-3">
                <div className="space-y-1.5">
                  <Label htmlFor="plan-price">Price (required)</Label>
                  <Input
                    id="plan-price"
                    name="price"
                    type="number"
                    min={0}
                    step="0.01"
                    required
                    defaultValue={plan ? (plan.price_cents / 100).toString() : ""}
                    placeholder="e.g. 499"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="plan-currency">Currency</Label>
                  <Input
                    id="plan-currency"
                    name="currency"
                    required
                    minLength={3}
                    maxLength={3}
                    defaultValue={plan?.currency ?? "GHS"}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="plan-interval">Interval</Label>
                  <Select
                    id="plan-interval"
                    name="billing_interval"
                    defaultValue={plan?.billing_interval ?? "monthly"}
                  >
                    <option value="monthly">Monthly</option>
                    <option value="yearly">Yearly</option>
                  </Select>
                </div>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="plan-features">Features (comma or newline separated)</Label>
                <Input
                  id="plan-features"
                  name="features"
                  defaultValue={plan?.features?.join(", ") ?? ""}
                  placeholder="e.g. student_management, attendance, analytics"
                />
              </div>

              {mode === "edit" ? (
                <div className="space-y-1.5">
                  <Label htmlFor="plan-status">Status</Label>
                  <Select id="plan-status" name="status" defaultValue={plan?.status ?? "active"}>
                    <option value="active">Active</option>
                    <option value="archived">Archived</option>
                  </Select>
                </div>
              ) : null}

              {state.error ? <p className="text-sm text-destructive">{state.error}</p> : null}

              <div className="flex items-center gap-3">
                <Button type="submit" loading={pending} loadingLabel="Saving">
                  {mode === "create" ? "Create plan" : "Save changes"}
                </Button>
                <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
                  Cancel
                </Button>
              </div>
            </form>
          </div>
        </div>
      </Sheet>
    </>
  );
}
