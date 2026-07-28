"use client";

import * as React from "react";
import { Trash2 } from "lucide-react";
import { Button } from "@auraedu/ui";
import { deletePlanAction, type BillingActionResult } from "@/lib/superadmin-billing-actions";

interface SuperadminDeletePlanButtonProps {
  planId: string;
  name: string;
}

export function SuperadminDeletePlanButton({ planId, name }: SuperadminDeletePlanButtonProps) {
  const [state, formAction, pending] = React.useActionState<BillingActionResult, FormData>(
    deletePlanAction.bind(null, planId),
    {},
  );

  return (
    <form action={formAction}>
      <Button
        type="submit"
        variant="ghost"
        loading={pending}
        loadingLabel="Deleting"
        onClick={(e) => {
          if (
            !confirm(
              `Delete plan "${name}"? Subscriptions on it are unaffected, but this cannot be undone.`,
            )
          ) {
            e.preventDefault();
          }
        }}
        className="h-8 px-2 text-destructive hover:bg-destructive/10"
      >
        <Trash2 className="size-4" />
        <span className="sr-only">Delete {name}</span>
      </Button>
      {state.error ? <span className="sr-only">{state.error}</span> : null}
    </form>
  );
}
