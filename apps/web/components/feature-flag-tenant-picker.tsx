"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Label, Select } from "@auraedu/ui";

export interface FeatureFlagTenantPickerProps {
  tenants: { tenant_code: string; name: string }[];
  selected: string;
}

/** Tenant picker for the flags grid — navigates with ?tenant= so the page stays server-rendered. */
export function FeatureFlagTenantPicker({ tenants, selected }: FeatureFlagTenantPickerProps) {
  const router = useRouter();

  return (
    <div className="flex items-center gap-2">
      <Label htmlFor="flags-tenant" className="text-sm text-muted-foreground">
        School
      </Label>
      <Select
        id="flags-tenant"
        className="h-9 w-56"
        value={selected}
        onChange={(e) =>
          router.push(`/superadmin/flags?tenant=${encodeURIComponent(e.target.value)}`)
        }
      >
        {tenants.map((t) => (
          <option key={t.tenant_code} value={t.tenant_code}>
            {t.name}
          </option>
        ))}
      </Select>
    </div>
  );
}
