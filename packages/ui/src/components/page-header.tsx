import * as React from "react";
import { cn } from "../lib/cn";

export interface PageHeaderProps {
  /** Leading lucide-style icon (decorative — the title carries meaning). */
  icon?: React.ReactNode;
  title: string;
  description?: string;
  /** Right-aligned primary action(s). */
  action?: React.ReactNode;
  className?: string;
}

/** Standard page header: brand-tinted icon chip + title + description + action (DESIGN_SYSTEM §9). */
export function PageHeader({ icon, title, description, action, className }: PageHeaderProps) {
  return (
    <div className={cn("flex items-start gap-3.5", className)}>
      {icon ? (
        <span
          aria-hidden="true"
          className="grid size-12 flex-none place-items-center rounded-[var(--radius-lg)] bg-[var(--accent)] text-[var(--primary)]"
        >
          {icon}
        </span>
      ) : null}
      <div className="min-w-0 flex-1">
        <h1 className="text-balance font-display text-2xl font-extrabold tracking-tight">
          {title}
        </h1>
        {description ? <p className="mt-1 text-sm text-muted-foreground">{description}</p> : null}
      </div>
      {action ? <div className="flex items-center gap-2">{action}</div> : null}
    </div>
  );
}
