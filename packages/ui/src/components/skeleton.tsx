import * as React from "react";
import { cn } from "../lib/cn";

/** Loading placeholder — mirrors real layout; never a spinner (DESIGN_SYSTEM §13). */
export function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      aria-hidden="true"
      className={cn("animate-pulse rounded-[var(--radius-sm)] bg-[var(--muted)]", className)}
      {...props}
    />
  );
}
