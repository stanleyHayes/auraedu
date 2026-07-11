"use client";

import * as React from "react";
import { PortalShell, type PortalShellProps } from "@/components/portal-shell";
import { logoutAction } from "@/lib/actions";

export type TeacherShellProps = Omit<PortalShellProps, "onLogout">;

export function TeacherShell(props: TeacherShellProps) {
  return <PortalShell {...props} onLogout={() => void logoutAction()} />;
}
