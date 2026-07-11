import * as React from "react";
import { cn } from "../lib/cn";

export interface EmptyStateProps {
  icon?: React.ReactNode;
  title: string;
  description?: string;
  actions?: React.ReactNode;
  className?: string;
}

/** Centre-aligned empty state with an animated floating icon (DESIGN_SYSTEM §14). */
export function EmptyState({ icon, title, description, actions, className }: EmptyStateProps) {
  return (
    <div
      role="status"
      className={cn(
        "flex flex-col items-center justify-center rounded-[var(--radius-md)] border border-dashed border-[var(--border)] bg-[var(--surface)] p-8 text-center",
        className,
      )}
    >
      {icon ? (
        <span
          aria-hidden="true"
          className="mb-4 text-[var(--muted-foreground)] motion-safe:animate-[empty-float_2.8s_var(--ease-out-quart)_infinite]"
        >
          {icon}
        </span>
      ) : null}
      <h3 className="font-sans text-lg font-semibold text-[var(--foreground)]">{title}</h3>
      {description ? (
        <p className="mt-2 max-w-sm text-sm text-[var(--muted-foreground)]">{description}</p>
      ) : null}
      {actions ? <div className="mt-5 flex items-center gap-3">{actions}</div> : null}
    </div>
  );
}
