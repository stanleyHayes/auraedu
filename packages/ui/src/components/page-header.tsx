"use client";

import * as React from "react";
import { cn } from "../lib/cn";
import { PageHelp, type PageGuide, useResolvedPageGuide } from "./page-guide";

export interface PageHeaderProps {
  /** Leading lucide-style icon (decorative). */
  icon?: React.ReactNode;
  title: string;
  description?: string;
  eyebrow?: string;
  /** Right-aligned primary action(s). */
  action?: React.ReactNode;
  /** Optional explicit guide; otherwise the application guide registry resolves it. */
  help?: PageGuide;
  className?: string;
}

/** Page header with a marketing-aligned signal rail and glass plaque. */
export function PageHeader({
  icon,
  title,
  description,
  eyebrow = "AuraEDU workspace",
  action,
  help,
  className,
}: PageHeaderProps) {
  const guide = useResolvedPageGuide(title, description, help);
  return (
    <div
      data-tour="page-header"
      className={cn(
        "portal-page-header relative overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-6 shadow-sm sm:p-7",
        "glass isolate",
        className,
      )}
    >
      <div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-[var(--portal-accent,var(--color-brand))] via-[var(--portal-accent-soft,var(--color-teal-bright))] to-[var(--portal-signal,var(--color-signal))]" />
      <span
        aria-hidden="true"
        className="absolute -left-20 -top-24 size-52 rounded-full bg-[var(--color-brand)]/10 blur-3xl"
      />
      {/* watermark motif */}
      <span
        aria-hidden="true"
        className="pointer-events-none absolute -right-3 -top-5 text-[var(--brand-text)] opacity-[0.05] motion-safe:animate-[float-mark_7s_ease-in-out_infinite]"
      >
        <span className="block size-36">{icon}</span>
      </span>
      {icon ? (
        <span
          aria-hidden="true"
          className="relative z-10 grid size-12 flex-none place-items-center rounded-xl border border-[var(--portal-accent,var(--color-brand))]/15 bg-[var(--accent)] text-[var(--portal-accent,var(--brand-text))] shadow-[0_10px_28px_color-mix(in_oklab,var(--portal-accent,var(--color-brand))_14%,transparent)]"
        >
          {icon}
        </span>
      ) : null}
      <div className="relative z-10 mt-4 flex items-start justify-between gap-4">
        <div className="min-w-0 flex-1">
          <p className="mb-2 font-mono text-[10px] font-black uppercase tracking-[0.18em] text-[var(--brand-text)]">
            {eyebrow}
          </p>
          <div className="flex items-center gap-2">
            <h1 className="text-balance font-heading text-2xl font-extrabold tracking-[-0.025em] text-[var(--foreground)] sm:text-3xl">
              {title}
            </h1>
            {guide ? <PageHelp guide={guide} /> : null}
          </div>
          {description ? (
            <p
              data-page-description
              className="mt-2 max-w-2xl text-sm leading-relaxed text-[var(--muted-foreground)] sm:text-[15px]"
            >
              {description}
            </p>
          ) : null}
        </div>
        {action ? (
          <div data-tour="primary-actions" className="flex shrink-0 items-center gap-2">
            {action}
          </div>
        ) : null}
      </div>
    </div>
  );
}
