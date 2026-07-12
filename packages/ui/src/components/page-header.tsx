import * as React from "react";
import { cn } from "../lib/cn";

export interface PageHeaderProps {
  /** Leading lucide-style icon (decorative). */
  icon?: React.ReactNode;
  title: string;
  description?: string;
  /** Right-aligned primary action(s). */
  action?: React.ReactNode;
  className?: string;
}

/** Page header with watermark icon, gold hairline, and glass plaque. */
export function PageHeader({ icon, title, description, action, className }: PageHeaderProps) {
  return (
    <div
      className={cn(
        "relative overflow-hidden rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface)] p-6 shadow-sm",
        "glass",
        className,
      )}
    >
      <div className="card-accent-strip absolute inset-x-0 top-0" />
      {/* watermark motif */}
      <span
        aria-hidden="true"
        className="pointer-events-none absolute -right-3 -top-5 text-[var(--color-brand)] opacity-[0.05] motion-safe:animate-[float-mark_7s_ease-in-out_infinite]"
      >
        <span className="block size-36">{icon}</span>
      </span>
      {icon ? (
        <span
          aria-hidden="true"
          className="relative z-10 grid size-11 flex-none place-items-center rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--accent)] text-[var(--primary)] shadow-sm"
        >
          {icon}
        </span>
      ) : null}
      <div className="relative z-10 mt-3 flex items-start justify-between gap-4">
        <div className="min-w-0 flex-1">
          <h1 className="text-balance font-heading text-2xl font-extrabold tracking-tight text-[var(--foreground)]">
            {title}
          </h1>
          {description ? (
            <p className="mt-1.5 text-sm leading-relaxed text-[var(--muted-foreground)]">
              {description}
            </p>
          ) : null}
        </div>
        {action ? <div className="flex shrink-0 items-center gap-2">{action}</div> : null}
      </div>
    </div>
  );
}
