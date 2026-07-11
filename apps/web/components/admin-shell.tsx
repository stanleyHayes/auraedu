"use client";

import * as React from "react";
import { PortalShell, type PortalShellProps } from "@/components/portal-shell";
import { logoutAction } from "@/lib/actions";

export type AdminShellProps = Omit<PortalShellProps, "onLogout">;

export function AdminShell(props: AdminShellProps) {
  return <PortalShell {...props} onLogout={() => logoutAction()} />;
}
