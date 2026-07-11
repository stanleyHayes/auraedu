"use client";

import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cn } from "../lib/cn";

export type ButtonVariant = "primary" | "secondary" | "ghost";

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  /** Shows the three-dot wave loader (DESIGN_SYSTEM §12) and sets aria-busy. */
  loading?: boolean;
  /** Announced to assistive tech while loading. */
  loadingLabel?: string;
  /** Render as the child element (e.g. Next.js Link) while keeping Button styles. */
  asChild?: boolean;
}

const base =
  "relative inline-flex h-10 items-center justify-center gap-2 rounded-[var(--radius-sm)] px-4 " +
  "text-sm font-bold transition-[transform,background-color,border-color] duration-150 " +
  "hover:-translate-y-px disabled:pointer-events-none disabled:opacity-60 " +
  "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--ring)]";

const variants: Record<ButtonVariant, string> = {
  primary: "bg-[var(--primary)] text-[var(--primary-foreground)] hover:brightness-95",
  secondary:
    "border border-[var(--border)] bg-[var(--surface)] text-[var(--foreground)] hover:bg-[var(--muted)]",
  ghost: "text-[var(--foreground)] hover:bg-[var(--muted)]",
};

/** Wave-dot loader — three dots rise in sequence; width stays stable (DESIGN_SYSTEM §12). */
function Wave({ label }: { label: string }) {
  return (
    <span
      className="absolute inset-0 inline-flex items-center justify-center"
      role="status"
      aria-live="polite"
    >
      <span className="sr-only">{label}</span>
      {[0, 1, 2].map((i) => (
        <span
          key={i}
          aria-hidden="true"
          className="mx-[3px] size-1.5 rounded-full bg-current motion-safe:animate-[button-wave_1s_ease-in-out_infinite]"
          style={{ animationDelay: `${i * 0.15}s` }}
        />
      ))}
    </span>
  );
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  {
    className,
    variant = "primary",
    loading = false,
    loadingLabel = "Working",
    asChild = false,
    children,
    disabled,
    ...props
  },
  ref,
) {
  const Comp = asChild ? Slot : "button";
  return (
    <Comp
      ref={ref}
      className={cn(base, variants[variant], className)}
      aria-busy={loading || undefined}
      disabled={asChild ? undefined : disabled ? true : loading}
      {...props}
    >
      {asChild ? (
        children
      ) : (
        <>
          {loading ? <Wave label={loadingLabel} /> : null}
          <span className={cn("inline-flex items-center gap-2", loading && "invisible")}>
            {children}
          </span>
        </>
      )}
    </Comp>
  );
});
