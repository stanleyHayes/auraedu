"use client";

import * as React from "react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import {
  createTemplateAction,
  updateTemplateAction,
  type AdminTemplateActionResult,
} from "@/lib/admin-template-actions";

type Template = OpenAPI.notification_v1.components["schemas"]["Template"];
type Channel = OpenAPI.notification_v1.components["schemas"]["Channel"];

const CHANNELS: { value: Channel; label: string }[] = [
  { value: "email", label: "Email" },
  { value: "sms", label: "SMS" },
  { value: "whatsapp", label: "WhatsApp" },
  { value: "in_app", label: "In app" },
  { value: "push", label: "Push" },
];

export function AdminTemplateForm({
  mode,
  initial,
  onSuccess,
}: {
  mode: "create" | "edit";
  initial?: Template;
  onSuccess?: () => void;
}) {
  const isEdit = mode === "edit";
  const action = isEdit ? updateTemplateAction.bind(null, initial!.id) : createTemplateAction;
  const [state, formAction, pending] = React.useActionState<AdminTemplateActionResult, FormData>(
    action,
    {},
  );
  const [channel, setChannel] = React.useState<Channel>(initial?.channel ?? "email");
  React.useEffect(() => {
    if (state.success) onSuccess?.();
  }, [state.success, onSuccess]);

  return (
    <form action={formAction} className="space-y-6">
      <div className="grid gap-5 sm:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="name">Template name</Label>
          <Input
            id="name"
            name="name"
            required
            maxLength={120}
            defaultValue={initial?.name}
            placeholder="Offer letter follow-up"
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="channel">Channel</Label>
          <Select
            id="channel"
            name="channel"
            value={channel}
            onChange={(event) => setChannel(event.target.value as Channel)}
          >
            {CHANNELS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </Select>
        </div>
        <div className="space-y-2 sm:col-span-2">
          <Label htmlFor="subject_template">
            Subject{" "}
            {channel === "email" ? null : (
              <span className="font-normal text-[var(--muted-foreground)]">
                (only used by email)
              </span>
            )}
          </Label>
          <Input
            id="subject_template"
            name="subject_template"
            maxLength={200}
            defaultValue={initial?.subject_template}
            placeholder="Your application to {{school_name}}"
            required={channel === "email"}
          />
        </div>
        <div className="space-y-2 sm:col-span-2">
          <Label htmlFor="body_template">Body</Label>
          <textarea
            id="body_template"
            name="body_template"
            required
            rows={8}
            defaultValue={initial?.body_template}
            placeholder="Hello {{first_name}}, …"
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm leading-6 outline-none transition focus:border-primary focus:ring-2 focus:ring-primary/20"
          />
          <p className="text-xs leading-5 text-[var(--muted-foreground)]">
            Personalise with variables in double braces, e.g. {"{{first_name}}"}. Only event context
            allowlisted by the sending workflow is available at render time — unknown variables fail
            safely instead of leaking data.
          </p>
        </div>
      </div>
      {state.error ? (
        <p role="alert" className="rounded-xl bg-red-500/10 px-4 py-3 text-sm text-red-700">
          {state.error}
        </p>
      ) : null}
      <div className="flex justify-end">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Creating"}>
          {isEdit ? "Save template" : "Create template"}
        </Button>
      </div>
    </form>
  );
}
