"use client";

import * as React from "react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import {
  updateTenantSettingsAction,
  type TenantActionResult,
} from "@/lib/tenant-actions";

interface TenantSettingsFormProps {
  tenantCode: string;
  initial?: OpenAPI.tenant_v1.components["schemas"]["Settings"];
}

export function TenantSettingsForm({ tenantCode, initial }: TenantSettingsFormProps) {
  const action = updateTenantSettingsAction.bind(null, tenantCode);
  const [state, formAction, pending] = React.useActionState<TenantActionResult, FormData>(
    action,
    {},
  );

  return (
    <form action={formAction} className="space-y-5">
      <div className="grid gap-5 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="locale">Locale</Label>
          <Input id="locale" name="locale" defaultValue={initial?.locale ?? "en-GH"} placeholder="en-GH" />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="timezone">Timezone</Label>
          <Input id="timezone" name="timezone" defaultValue={initial?.timezone ?? "Africa/Accra"} placeholder="Africa/Accra" />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="date_format">Date format</Label>
          <Select id="date_format" name="date_format" defaultValue={initial?.date_format ?? "DD/MM/YYYY"}>
            <option value="DD/MM/YYYY">DD/MM/YYYY</option>
            <option value="MM/DD/YYYY">MM/DD/YYYY</option>
            <option value="YYYY-MM-DD">YYYY-MM-DD</option>
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="academic_year_start_month">Academic year starts (month)</Label>
          <Select
            id="academic_year_start_month"
            name="academic_year_start_month"
            defaultValue={String(initial?.academic_year_start_month ?? 9)}
          >
            {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5 sm:col-span-2">
          <Label htmlFor="primary_contact_email">Primary contact email</Label>
          <Input
            id="primary_contact_email"
            name="primary_contact_email"
            type="email"
            defaultValue={initial?.primary_contact_email ?? ""}
            placeholder="admin@school.edu"
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
          Settings saved.
        </p>
      ) : null}

      <div className="flex justify-end gap-3 pt-2">
        <Button type="submit" loading={pending} loadingLabel="Saving">
          Save settings
        </Button>
      </div>
    </form>
  );
}
