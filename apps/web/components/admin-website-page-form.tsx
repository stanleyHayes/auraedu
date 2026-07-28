"use client";

import * as React from "react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import {
  createPageAction,
  updatePageAction,
  type AdminWebsiteActionResult,
} from "@/lib/admin-website-actions";

type WebsitePage = OpenAPI.website_v1.components["schemas"]["Page"];

interface AdminWebsitePageFormProps {
  mode: "create" | "edit";
  pageId?: string;
  initial?: WebsitePage;
  onSuccess?: () => void;
}

export function AdminWebsitePageForm({
  mode,
  pageId,
  initial,
  onSuccess,
}: AdminWebsitePageFormProps) {
  const isEdit = mode === "edit";
  const action = isEdit ? updatePageAction.bind(null, pageId!) : createPageAction;

  const [state, formAction, pending] = React.useActionState<AdminWebsiteActionResult, FormData>(
    action,
    {},
  );

  React.useEffect(() => {
    if (state.success && onSuccess) {
      onSuccess();
    }
  }, [state, onSuccess]);

  return (
    <form action={formAction} className="space-y-5">
      <div className="grid gap-5 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="title">Page title</Label>
          <Input
            id="title"
            name="title"
            defaultValue={initial?.title}
            required
            placeholder="Admissions"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="slug">URL slug</Label>
          <Input
            id="slug"
            name="slug"
            defaultValue={initial?.slug}
            required
            pattern="[a-z0-9-]+"
            placeholder="admissions"
          />
          <p className="text-xs text-muted-foreground">
            Lowercase letters, numbers and hyphens. The homepage uses the slug &quot;home&quot;.
          </p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="layout">Layout</Label>
          <Select id="layout" name="layout" defaultValue={initial?.layout ?? "default"}>
            <option value="default">Default</option>
            <option value="landing">Landing</option>
            <option value="contact">Contact</option>
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="status">Status</Label>
          <Select id="status" name="status" defaultValue={initial?.status ?? "draft"}>
            <option value="draft">Draft</option>
            <option value="published">Published</option>
            <option value="archived">Archived</option>
          </Select>
        </div>

        <div className="space-y-1.5 sm:col-span-2">
          <Label htmlFor="meta_description">Search description</Label>
          <textarea
            id="meta_description"
            name="meta_description"
            rows={2}
            defaultValue={initial?.meta_description ?? ""}
            className="w-full rounded-xl border border-border bg-background p-3 text-sm"
            placeholder="Shown in search results and link previews."
          />
        </div>
      </div>

      {state.error ? (
        <p className="rounded-[var(--radius-sm)] bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {state.error}
        </p>
      ) : null}
      {state.success ? (
        <p className="rounded-[var(--radius-sm)] bg-emerald-500/10 px-3 py-2 text-sm text-emerald-600">
          {isEdit ? "Page saved." : "Page created."}
        </p>
      ) : null}

      <div className="flex justify-end gap-3 pt-2">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Creating"}>
          {isEdit ? "Save changes" : "Create page"}
        </Button>
      </div>
    </form>
  );
}
