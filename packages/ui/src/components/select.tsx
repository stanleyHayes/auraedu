"use client";

import * as React from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "../lib/cn";

export type SelectProps = React.SelectHTMLAttributes<HTMLSelectElement>;

export const Select = React.forwardRef<HTMLSelectElement, SelectProps>(function Select(
  { className, children, ...props },
  ref,
) {
  return (
    <div className="relative">
      <select
        ref={ref}
        className={cn(
          "h-11 w-full appearance-none rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--input)] px-3.5 pr-9 text-sm text-[var(--foreground)] shadow-sm",
          "focus-visible:border-[var(--portal-accent,var(--color-brand))] focus-visible:bg-[var(--input-focus)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]/40",
          "disabled:cursor-not-allowed disabled:opacity-60",
          "transition-[border-color,background-color,box-shadow] duration-150",
          className,
        )}
        {...props}
      >
        {children}
      </select>
      <ChevronDown
        className="pointer-events-none absolute right-3 top-1/2 size-4 -translate-y-1/2 text-[var(--muted-foreground)]"
        aria-hidden="true"
      />
    </div>
  );
});
