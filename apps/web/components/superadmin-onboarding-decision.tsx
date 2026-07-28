"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Check, X } from "lucide-react";
import { Button, Input, Label, Sheet } from "@auraedu/ui";
import {
  approveOnboardingRequestAction,
  rejectOnboardingRequestAction,
  type OnboardingActionResult,
} from "@/lib/superadmin-onboarding-actions";

export interface SuperadminOnboardingDecisionProps {
  requestId: string;
  schoolName: string;
}

/**
 * Approve / reject controls for a pending onboarding request. Both decisions open a
 * confirmation sheet: approval requires the tenant code to provision, rejection
 * requires a reason (contract tenant.v1 mandates both).
 */
export function SuperadminOnboardingDecision({
  requestId,
  schoolName,
}: SuperadminOnboardingDecisionProps) {
  const [sheet, setSheet] = React.useState<"approve" | "reject" | null>(null);

  return (
    <>
      <div className="flex items-center gap-2">
        <Button
          type="button"
          variant="ghost"
          className="h-8 px-2 text-[var(--color-ok)] hover:bg-[var(--color-ok)]/10"
          onClick={() => setSheet("approve")}
        >
          <Check className="size-4" />
          <span className="sr-only">Approve {schoolName}</span>
        </Button>
        <Button
          type="button"
          variant="ghost"
          className="h-8 px-2 text-destructive hover:bg-destructive/10"
          onClick={() => setSheet("reject")}
        >
          <X className="size-4" />
          <span className="sr-only">Reject {schoolName}</span>
        </Button>
      </div>

      <DecisionSheet
        kind="approve"
        open={sheet === "approve"}
        onClose={() => setSheet(null)}
        requestId={requestId}
        schoolName={schoolName}
      />
      <DecisionSheet
        kind="reject"
        open={sheet === "reject"}
        onClose={() => setSheet(null)}
        requestId={requestId}
        schoolName={schoolName}
      />
    </>
  );
}

interface DecisionSheetProps {
  kind: "approve" | "reject";
  open: boolean;
  onClose: () => void;
  requestId: string;
  schoolName: string;
}

function DecisionSheet({ kind, open, onClose, requestId, schoolName }: DecisionSheetProps) {
  const router = useRouter();
  const serverAction =
    kind === "approve" ? approveOnboardingRequestAction : rejectOnboardingRequestAction;
  const [state, formAction, pending] = React.useActionState<OnboardingActionResult, FormData>(
    serverAction.bind(null, requestId),
    {},
  );

  React.useEffect(() => {
    if (state.success) {
      onClose();
      router.refresh();
    }
  }, [state, router, onClose]);

  return (
    <Sheet
      open={open}
      onClose={onClose}
      side="right"
      className="w-full max-w-md bg-[var(--surface)] p-0"
    >
      <div className="flex h-full flex-col">
        <div className="border-b border-[var(--border)] bg-[var(--muted)] px-6 py-4">
          <h2 className="font-heading text-lg font-bold">
            {kind === "approve" ? "Approve" : "Reject"} onboarding request
          </h2>
          <p className="text-sm text-muted-foreground">
            {kind === "approve"
              ? `Provision a tenant for ${schoolName}. The tenant code becomes the school's subdomain and cannot be changed later.`
              : `Reject the request from ${schoolName}. The reason is recorded with the decision.`}
          </p>
        </div>
        <div className="flex-1 overflow-y-auto p-6">
          <form action={formAction} className="space-y-5">
            {kind === "approve" ? (
              <div className="space-y-1.5">
                <Label htmlFor={`tenant-code-${requestId}`}>Tenant code (required)</Label>
                <Input
                  id={`tenant-code-${requestId}`}
                  name="tenant_code"
                  required
                  pattern="[a-z0-9-]{2,50}"
                  placeholder="e.g. ridge-view-academy"
                />
                <p className="text-xs text-muted-foreground">
                  2–50 lowercase letters, numbers, or hyphens.
                </p>
              </div>
            ) : (
              <div className="space-y-1.5">
                <Label htmlFor={`reason-${requestId}`}>Rejection reason (required)</Label>
                <Input
                  id={`reason-${requestId}`}
                  name="reason"
                  required
                  minLength={3}
                  maxLength={500}
                  placeholder="e.g. Duplicate request for an existing school"
                />
              </div>
            )}

            {state.error ? <p className="text-sm text-destructive">{state.error}</p> : null}

            <div className="flex items-center gap-3">
              <Button
                type="submit"
                loading={pending}
                loadingLabel={kind === "approve" ? "Approving" : "Rejecting"}
                variant={kind === "approve" ? "primary" : "secondary"}
              >
                {kind === "approve" ? "Approve & provision" : "Reject request"}
              </Button>
              <Button type="button" variant="ghost" onClick={onClose}>
                Cancel
              </Button>
            </div>
          </form>
        </div>
      </div>
    </Sheet>
  );
}
