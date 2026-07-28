"use client";

import * as React from "react";
import { Trash2 } from "lucide-react";
import { Button } from "@auraedu/ui";
import {
  deleteReportTemplateAction,
  type AdminReportActionResult,
} from "@/lib/admin-report-actions";

interface AdminReportTemplateDeleteButtonProps {
  id: string;
  name: string;
}

export function AdminReportTemplateDeleteButton({
  id,
  name,
}: AdminReportTemplateDeleteButtonProps) {
  const [state, formAction, pending] = React.useActionState<AdminReportActionResult, FormData>(
    deleteReportTemplateAction.bind(null, id),
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
          if (!confirm(`Delete "${name}"? Cards already generated from it are kept.`)) {
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
