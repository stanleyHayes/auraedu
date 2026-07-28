"use client";

import * as React from "react";
import { Archive, FileDown, PlayCircle, Send } from "lucide-react";
import { Button } from "@auraedu/ui";
import {
  archiveReportCardAction,
  generateReportCardAction,
  publishReportCardAction,
  type AdminReportActionResult,
} from "@/lib/admin-report-actions";

type CardAction = (id: string) => Promise<AdminReportActionResult>;

function CardActionButton({
  id,
  action,
  label,
  loadingLabel,
  icon,
  variant = "secondary",
  confirmMessage,
}: {
  id: string;
  action: CardAction;
  label: string;
  loadingLabel: string;
  icon: React.ReactNode;
  variant?: "primary" | "secondary" | "ghost";
  confirmMessage?: string;
}) {
  const [state, formAction, pending] = React.useActionState<AdminReportActionResult, FormData>(
    action.bind(null, id),
    {},
  );

  return (
    <form action={formAction} className="inline-flex items-center gap-2">
      <Button
        type="submit"
        variant={variant}
        loading={pending}
        loadingLabel={loadingLabel}
        onClick={(e) => {
          if (confirmMessage && !confirm(confirmMessage)) {
            e.preventDefault();
          }
        }}
        className="gap-1.5"
      >
        {icon}
        {label}
      </Button>
      {state.error ? <span className="sr-only">{state.error}</span> : null}
    </form>
  );
}

interface AdminReportCardActionsProps {
  id: string;
  status: string;
}

export function AdminReportCardActions({ id, status }: AdminReportCardActionsProps) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      {status === "draft" ? (
        <CardActionButton
          id={id}
          action={generateReportCardAction}
          label="Generate"
          loadingLabel="Starting"
          icon={<PlayCircle className="size-4" />}
          variant="primary"
        />
      ) : null}
      {status === "draft" || status === "generating" ? (
        <CardActionButton
          id={id}
          action={publishReportCardAction}
          label="Publish"
          loadingLabel="Publishing"
          icon={<Send className="size-4" />}
          confirmMessage="Publish this report card? It becomes visible to guardians."
        />
      ) : null}
      {status === "published" ? (
        <>
          <a
            className="inline-flex items-center gap-1.5 font-bold text-[var(--primary)] hover:underline"
            href={`/api/reports/${id}/download`}
          >
            <FileDown className="size-4" />
            Download
          </a>
          <CardActionButton
            id={id}
            action={archiveReportCardAction}
            label="Archive"
            loadingLabel="Archiving"
            icon={<Archive className="size-4" />}
            variant="ghost"
          />
        </>
      ) : null}
    </div>
  );
}
