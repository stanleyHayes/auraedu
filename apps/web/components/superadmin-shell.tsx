"use client";

import * as React from "react";
import { PortalShell, type PortalShellProps } from "@/components/portal-shell";
import { logoutAction } from "@/lib/actions";

export type SuperAdminShellProps = Omit<PortalShellProps, "onLogout">;

export function SuperAdminShell(props: SuperAdminShellProps) {
  return <PortalShell {...props} onLogout={() => logoutAction()} />;
}
