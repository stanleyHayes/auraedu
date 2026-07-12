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

/** KPI tile for the role-aware Overview with gold corner motif. */
export function StatCard({ label, value, unit, tone = "default", className }: StatCardProps) {
  return (
    <div
      className={cn(
        "relative overflow-hidden rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-4 transition-transform duration-200 hover:-translate-y-0.5 hover:shadow-sm",
        className,
      )}
    >
      <span
        aria-hidden="true"
        className="absolute -right-3 -top-3 size-12 rounded-full bg-[var(--color-gold)]/8"
      />
      <div className="font-mono text-[10.5px] uppercase tracking-[0.1em] text-[var(--muted-foreground)]">
        {label}
      </div>
      <div
        className={cn("mt-1.5 text-[28px] font-extrabold tracking-[-0.02em]", toneClass[tone])}
        style={{ fontFamily: "var(--font-sans)" }}
      >
        {value}
        {unit ? (
          <small className="text-sm font-bold text-[var(--muted-foreground)]"> {unit}</small>
        ) : null}
      </div>
    </div>
  );
}
