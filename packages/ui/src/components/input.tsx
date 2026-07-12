"use client";

import * as React from "react";
import { cn } from "../lib/cn";

export type InputProps = React.InputHTMLAttributes<HTMLInputElement>;

export const Input = React.forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, ...props },
  ref,
) {
  return (
    <input
      ref={ref}
      className={cn(
        "h-11 w-full rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--color-parchment)] px-3.5 text-sm text-[var(--foreground)] shadow-sm",
        "placeholder:text-[var(--muted-foreground)]",
        "focus-visible:border-[var(--color-gold)] focus-visible:bg-[var(--surface)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]/40",
        "disabled:cursor-not-allowed disabled:opacity-60",
        "transition-[border-color,background-color,box-shadow] duration-150",
        className,
      )}
      {...props}
    />
  );
})
