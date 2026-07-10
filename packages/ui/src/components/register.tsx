"use client";

import * as React from "react";
import { cn } from "../lib/cn";

export interface Pupil {
  id: string;
  name: string;
  present: boolean;
}

export interface RegisterCardProps {
  /** e.g. "Form 2 Science · Register" */
  title: string;
  /** e.g. "Mon 10 Jul · 08:05" */
  meta?: string;
  pupils: Pupil[];
  /** Class size when `pupils` is an excerpt; defaults to `pupils.length`. */
  total?: number;
  onToggle?: (id: string) => void;
  className?: string;
}

function Tick() {
  return (
    <svg viewBox="0 0 16 12" className="size-3.5" aria-hidden="true">
      <path
        d="M1 6.5 5.2 10.5 15 1"
        fill="none"
        stroke="currentColor"
        strokeWidth={2.4}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

const label = "font-mono text-[11px] uppercase tracking-[0.14em]";

/**
 * The AuraEDU signature component (BRAND.md §4): the attendance register.
 * The ruled rows and the red tick are the brand made operable. Tap a mark to
 * toggle present/absent; the accent is the tenant's brand colour (via --primary).
 */
export function RegisterCard({ title, meta, pupils, total, onToggle, className }: RegisterCardProps) {
  const present = pupils.filter((p) => p.present).length;
  const shownTotal = total ?? pupils.length;

  return (
    <div
      className={cn(
        "overflow-hidden rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)]",
        className,
      )}
    >
      <div className="flex items-center justify-between border-b-2 border-[var(--foreground)] bg-[var(--muted)] px-4 py-3">
        <span className={cn(label, "text-[var(--foreground)]")}>{title}</span>
        {meta ? <span className={cn(label, "text-[var(--muted-foreground)]")}>{meta}</span> : null}
      </div>

      <ol className="relative m-0 list-none p-0">
        {pupils.map((p, i) => (
          <li
            key={p.id}
            data-state={p.present ? "present" : "absent"}
            className="grid h-12 grid-cols-[52px_1fr_auto] items-center border-b border-[var(--border)] pr-4"
          >
            <span className="text-center font-mono text-xs tabular-nums text-[var(--muted-foreground)]">
              {String(i + 1).padStart(2, "0")}
            </span>
            <span className={cn("text-sm font-medium", !p.present && "text-[var(--muted-foreground)]")}>
              {p.name}
            </span>
            <button
              type="button"
              onClick={() => onToggle?.(p.id)}
              aria-pressed={p.present}
              aria-label={`Mark ${p.name} ${p.present ? "absent" : "present"}`}
              className={cn(
                "grid size-6 place-items-center rounded-[4px] border bg-[var(--background)] text-[var(--primary)] transition-colors",
                p.present ? "border-[var(--primary)]" : "border-[var(--border)] hover:border-[var(--primary)]",
              )}
            >
              {p.present ? <Tick /> : <span className="h-0.5 w-2.5 rounded bg-[var(--muted-foreground)]" />}
            </button>
          </li>
        ))}
      </ol>

      <div className="flex items-center justify-between border-t-2 border-[var(--foreground)] bg-[var(--muted)] px-4 py-3">
        <span className={cn(label, "text-[var(--muted-foreground)]")}>Present</span>
        <span className="font-mono text-sm tabular-nums">
          <b className="text-lg text-[var(--primary)]">{present}</b> / {shownTotal}
        </span>
      </div>
    </div>
  );
}
