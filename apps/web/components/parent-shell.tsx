"use client";

import * as React from "react";
import { PortalShell, type PortalShellProps } from "@/components/portal-shell";
import { logoutAction } from "@/lib/actions";

export type ParentShellProps = Omit<PortalShellProps, "onLogout">;

export function ParentShell(props: ParentShellProps) {
  return <PortalShell {...props} onLogout={() => void logoutAction()} />;
}
