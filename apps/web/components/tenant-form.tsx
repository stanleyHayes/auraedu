"use client";

import * as React from "react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import {
  createTenantAction,
  updateTenantAction,
  type TenantActionResult,
} from "@/lib/tenant-actions";
import { LogoUploader } from "./logo-uploader";

const PLAN_OPTIONS: OpenAPI.tenant_v1.components["schemas"]["Tenant"]["plan"][] = [
  "starter",
  "growth",
  "professional",
  "ai_plus",
  "enterprise",
];

const STATUS_OPTIONS: OpenAPI.tenant_v1.components["schemas"]["Tenant"]["status"][] = [
  "active",
  "suspended",
  "onboarding",
];

interface TenantFormProps {
  mode: "create" | "edit";
  tenantCode?: string;
  initial?: OpenAPI.tenant_v1.components["schemas"]["Tenant"];
  onSuccess?: () => void;
}

export function TenantForm({ mode, tenantCode, initial, onSuccess }: TenantFormProps) {
  const isEdit = mode === "edit";
  const action = isEdit
    ? updateTenantAction.bind(null, tenantCode!)
    : createTenantAction;

  const [state, formAction, pending] = React.useActionState<TenantActionResult, FormData>(
    action,
    {},
  );

  const [logoUrl, setLogoUrl] = React.useState<string | null>(
    initial?.branding?.logo_url ?? null,
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
          <Label htmlFor="tenant_code">Tenant code</Label>
          <Input
            id="tenant_code"
            name="tenant_code"
            defaultValue={initial?.tenant_code}
            disabled={isEdit}
            required
            pattern="^[a-z0-9-]{2,50}$"
            placeholder="upshs"
            title="2–50 lowercase letters, numbers, or hyphens"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="name">School name</Label>
          <Input id="name" name="name" defaultValue={initial?.name} required placeholder="Union Preparatory School" />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="short">Short name</Label>
          <Input id="short" name="short" defaultValue={initial?.short} placeholder="UPSHS" />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="domain">Custom domain</Label>
          <Input id="domain" name="domain" defaultValue={initial?.domain ?? ""} placeholder="upshs.auraedu.com" />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="status">Status</Label>
          <Select id="status" name="status" defaultValue={initial?.status ?? "active"}>
            {STATUS_OPTIONS.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="plan">Plan</Label>
          <Select id="plan" name="plan" defaultValue={initial?.plan ?? "starter"}>
            {PLAN_OPTIONS.map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="brand_primary">Primary brand colour</Label>
          <input
            id="brand_primary"
            name="brand_primary"
            type="color"
            defaultValue={initial?.branding?.brand?.primary ?? "#C6402F"}
            className="h-11 w-full rounded-[var(--radius-sm)] border border-border bg-surface px-2 py-1"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="brand_secondary">Secondary brand colour</Label>
          <input
            id="brand_secondary"
            name="brand_secondary"
            type="color"
            defaultValue={initial?.branding?.brand?.secondary ?? "#2456A6"}
            className="h-11 w-full rounded-[var(--radius-sm)] border border-border bg-surface px-2 py-1"
          />
        </div>
      </div>

      <input type="hidden" name="logo_url" value={logoUrl ?? ""} />
      <LogoUploader tenantCode={initial?.tenant_code ?? tenantCode ?? "new"} value={logoUrl} onChange={setLogoUrl} disabled={pending} />

      {state.error ? (
        <p className="rounded-[var(--radius-sm)] bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {state.error}
        </p>
      ) : null}
      {state.success ? (
        <p className="rounded-[var(--radius-sm)] bg-emerald-500/10 px-3 py-2 text-sm text-emerald-600">
          {isEdit ? "Tenant saved." : "School created."}
        </p>
      ) : null}

      <div className="flex justify-end gap-3 pt-2">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Creating"}>
          {isEdit ? "Save changes" : "Create school"}
        </Button>
      </div>
    </form>
  );
}
