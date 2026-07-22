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

/** KPI tile for the role-aware overview with the shared signal motif. */
export function StatCard({ label, value, unit, tone = "default", className }: StatCardProps) {
  return (
    <div
      className={cn(
        "portal-stat-card group relative overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-5 transition-[transform,box-shadow,border-color] duration-300 hover:-translate-y-1 hover:border-[var(--portal-accent,var(--color-brand))]/25 hover:shadow-[0_18px_42px_rgba(6,22,49,0.09)]",
        className,
      )}
    >
      <span
        aria-hidden="true"
        className="absolute -right-7 -top-7 size-24 rounded-full bg-gradient-to-br from-[var(--portal-accent,var(--color-brand))]/12 to-[var(--portal-signal,var(--color-signal))]/10 transition-transform duration-500 group-hover:scale-125"
      />
      <span
        aria-hidden="true"
        className="absolute inset-y-5 left-0 w-[3px] rounded-r-full bg-gradient-to-b from-[var(--portal-accent,var(--color-brand))] to-[var(--portal-accent-soft,var(--color-teal-bright))]"
      />
      <div className="font-mono text-[9.5px] font-bold uppercase tracking-[0.16em] text-[var(--muted-foreground)]">
        {label}
      </div>
      <div
        className={cn("mt-2 text-[30px] font-black tracking-[-0.035em]", toneClass[tone])}
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
