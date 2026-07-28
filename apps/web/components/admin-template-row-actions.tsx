"use client";

import * as React from "react";
import { Archive, ArchiveRestore, Trash2 } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button } from "@auraedu/ui";
import {
  deleteTemplateAction,
  setTemplateStatusAction,
  type AdminTemplateActionResult,
} from "@/lib/admin-template-actions";

type Template = OpenAPI.notification_v1.components["schemas"]["Template"];

export function AdminTemplateRowActions({ template }: { template: Template }) {
  const isActive = template.status === "active";
  const toggle = React.useMemo(
    () => setTemplateStatusAction.bind(null, template.id, isActive ? "archived" : "active"),
    [template.id, isActive],
  );
  const remove = React.useMemo(() => deleteTemplateAction.bind(null, template.id), [template.id]);
  const [toggleState, toggleAction, togglePending] = React.useActionState<
    AdminTemplateActionResult,
    FormData
  >(toggle, {});
  const [deleteState, deleteAction, deletePending] = React.useActionState<
    AdminTemplateActionResult,
    FormData
  >(remove, {});

  const error = toggleState.error ?? deleteState.error;

  return (
    <div className="flex flex-col items-end gap-2">
      <div className="flex items-center justify-end gap-1">
        <form action={toggleAction}>
          <Button
            type="submit"
            variant="ghost"
            className="h-8 gap-1.5 px-2 text-xs"
            loading={togglePending}
            loadingLabel={isActive ? "Archiving" : "Activating"}
          >
            {isActive ? <Archive className="size-4" /> : <ArchiveRestore className="size-4" />}
            {isActive ? "Archive" : "Activate"}
          </Button>
        </form>
        <form
          action={deleteAction}
          onSubmit={(event) => {
            if (
              !window.confirm(
                `Delete "${template.name}" permanently? Journeys or sends that reference it will fail until they pick another template.`,
              )
            ) {
              event.preventDefault();
            }
          }}
        >
          <Button
            type="submit"
            variant="ghost"
            className="h-8 px-2 text-destructive hover:bg-destructive/10"
            loading={deletePending}
            loadingLabel="Deleting"
          >
            <Trash2 className="size-4" />
            <span className="sr-only">Delete {template.name}</span>
          </Button>
        </form>
      </div>
      {error ? (
        <p role="alert" className="max-w-md text-right text-xs leading-5 text-red-700">
          {error}
        </p>
      ) : null}
    </div>
  );
}
