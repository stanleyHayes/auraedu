"use client";

import * as React from "react";
import { cn } from "../lib/cn";

export interface LabelProps extends React.LabelHTMLAttributes<HTMLLabelElement> {
  required?: boolean;
}

export const Label = React.forwardRef<HTMLLabelElement, LabelProps>(function Label(
  { className, children, required, ...props },
  ref,
) {
  return (
    <label
      ref={ref}
      className={cn(
        "mb-1.5 block text-sm font-semibold text-[var(--foreground)]",
        className,
      )}
      {...props}
    >
      {children}
      {required ? <span className="ml-0.5 text-[var(--color-crit)]">*</span> : null}
    </label>
  );
});
