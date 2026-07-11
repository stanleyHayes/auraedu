"use client";

import * as React from "react";
import { PortalShell, type PortalShellProps } from "@/components/portal-shell";
import { logoutAction } from "@/lib/actions";

export type StudentShellProps = Omit<PortalShellProps, "onLogout">;

export function StudentShell(props: StudentShellProps) {
  return <PortalShell {...props} onLogout={() => logoutAction()} />;
}
