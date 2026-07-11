import * as React from "react";
import { cn } from "../lib/cn";

export interface StatCardProps {
  label: string;
  value: React.ReactNode;
  unit?: string;
  tone?: "default" | "warn" | "ok";
  className?: string;
}

const toneClass: Record<NonNullable<StatCardProps["tone"]>, string> = {
  default: "text-[var(--foreground)]",
  warn: "text-[var(--color-warn)]",
  ok: "text-[var(--color-ok)]",
};

/** KPI tile for the role-aware Overview (DESIGN_SYSTEM §16). */
export function StatCard({ label, value, unit, tone = "default", className }: StatCardProps) {
  return (
    <div
      className={cn(
        "rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-4",
        className,
      )}
    >
      <div className="font-mono text-[10.5px] uppercase tracking-[0.1em] text-[var(--muted-foreground)]">
        {label}
      </div>
      <div
        className={cn("mt-1.5 text-[28px] font-extrabold tracking-[-0.02em]", toneClass[tone])}
        style={{ fontFamily: "var(--font-display)" }}
      >
        {value}
        {unit ? (
          <small className="text-sm font-bold text-[var(--muted-foreground)]"> {unit}</small>
        ) : null}
      </div>
    </div>
  );
}
