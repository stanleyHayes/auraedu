import * as React from "react";
import { cn } from "../lib/cn";

export interface EmptyStateProps {
  icon?: React.ReactNode;
  title: string;
  description?: string;
  actions?: React.ReactNode;
  className?: string;
}

/** Centre-aligned empty state with floating icon and corner motif. */
export function EmptyState({ icon, title, description, actions, className }: EmptyStateProps) {
  return (
    <div
      role="status"
      className={cn(
        "relative flex flex-col items-center justify-center overflow-hidden rounded-[var(--radius-lg)] border border-dashed border-[var(--border)] bg-[var(--surface)] p-10 text-center",
        className,
      )}
    >
      <span
        aria-hidden="true"
        className="absolute left-0 top-0 h-12 w-12 -translate-x-1/2 -translate-y-1/2 rounded-full bg-[var(--color-gold)]/10"
      />
      <span
        aria-hidden="true"
        className="absolute bottom-0 right-0 h-16 w-16 translate-x-1/3 translate-y-1/3 rounded-full bg-[var(--color-brand)]/8"
      />
      {icon ? (
        <span
          aria-hidden="true"
          className="relative z-10 mb-5 text-[var(--muted-foreground)] motion-safe:animate-[empty-float_2.8s_var(--ease-out-quart)_infinite]"
        >
          <span className="block size-10">{icon}</span>
        </span>
      ) : null}
      <h3 className="relative z-10 font-heading text-lg font-semibold text-[var(--foreground)]">
        {title}
      </h3>
      {description ? (
        <p className="relative z-10 mt-2 max-w-sm text-sm leading-relaxed text-[var(--muted-foreground)]">
          {description}
        </p>
      ) : null}
      {actions ? <div className="relative z-10 mt-6 flex items-center gap-3">{actions}</div> : null}
    </div>
  );
}
