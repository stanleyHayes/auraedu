"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Label, Select } from "@auraedu/ui";

export interface SuperadminAuditTenantPickerProps {
  tenants: { tenant_code: string; name: string }[];
  /** Selected tenant code, or "" for the cross-tenant "All tenants" view. */
  selected: string;
}

/**
 * Tenant picker for the audit explorer — navigates with ?tenant= so the page stays
 * server-rendered. The empty "All tenants" option omits the tenant pin entirely.
 */
export function SuperadminAuditTenantPicker({
  tenants,
  selected,
}: SuperadminAuditTenantPickerProps) {
  const router = useRouter();

  return (
    <div className="flex items-center gap-2">
      <Label htmlFor="audit-tenant" className="text-sm text-muted-foreground">
        School
      </Label>
      <Select
        id="audit-tenant"
        className="h-9 w-56"
        value={selected}
        onChange={(e) => {
          const code = e.target.value;
          router.push(
            code
              ? `/superadmin/audit-logs?tenant=${encodeURIComponent(code)}`
              : "/superadmin/audit-logs",
          );
        }}
      >
        <option value="">All tenants</option>
        {tenants.map((t) => (
          <option key={t.tenant_code} value={t.tenant_code}>
            {t.name}
          </option>
        ))}
      </Select>
    </div>
  );
}
