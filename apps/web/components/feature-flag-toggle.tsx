"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Button, Input, Label, Sheet } from "@auraedu/ui";
import {
  overrideFeatureFlagAction,
  type FeatureFlagActionResult,
} from "@/lib/feature-flag-actions";

export interface FeatureFlagToggleProps {
  tenantCode: string;
  tenantName: string;
  featureKey: string;
  enabled: boolean;
}

/**
 * Switch for one tenant feature flag. Opens a sheet asking for the override reason
 * (required by POST /api/v1/super-admin/features/{key}/override) before submitting.
 */
export function FeatureFlagToggle({
  tenantCode,
  tenantName,
  featureKey,
  enabled,
}: FeatureFlagToggleProps) {
  const router = useRouter();
  const [open, setOpen] = React.useState(false);

  const nextEnabled = !enabled;
  const action = overrideFeatureFlagAction.bind(null, tenantCode, featureKey, nextEnabled);
  const [state, formAction, pending] = React.useActionState<FeatureFlagActionResult, FormData>(
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
      <button
        type="button"
        role="switch"
        aria-checked={enabled}
        aria-label={`${enabled ? "Disable" : "Enable"} ${featureKey} for ${tenantName}`}
        onClick={() => setOpen(true)}
        className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--ring)] ${
          enabled ? "bg-[var(--color-ok)]" : "bg-[var(--muted)]"
        }`}
      >
        <span
          aria-hidden="true"
          className={`inline-block size-4 transform rounded-full bg-[var(--surface)] shadow transition-transform ${
            enabled ? "translate-x-6" : "translate-x-1"
          }`}
        />
      </button>

      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-md bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <div className="border-b border-[var(--border)] bg-[var(--muted)] px-6 py-4">
            <h2 className="font-heading text-lg font-bold">
              {nextEnabled ? "Enable" : "Disable"} feature
            </h2>
            <p className="text-sm text-muted-foreground">
              Platform override for <span className="font-mono text-xs">{featureKey}</span> on{" "}
              {tenantName}. The reason is stored with the flag for audit.
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <form action={formAction} className="space-y-5">
              <div className="space-y-1.5">
                <Label htmlFor="reason">Reason (required)</Label>
                <Input
                  id="reason"
                  name="reason"
                  required
                  placeholder="e.g. Support ticket SUP-123 — enable analytics pilot"
                />
              </div>

              {state.error ? <p className="text-sm text-destructive">{state.error}</p> : null}

              <div className="flex items-center gap-3">
                <Button type="submit" loading={pending} loadingLabel="Saving">
                  {nextEnabled ? "Enable feature" : "Disable feature"}
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
