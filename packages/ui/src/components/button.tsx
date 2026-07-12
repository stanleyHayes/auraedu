"use client";

import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cn } from "../lib/cn";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "gold" | "navy";

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  /** Pill-shaped button. */
  pill?: boolean;
  /** Shows the three-dot wave loader and sets aria-busy. */
  loading?: boolean;
  /** Announced to assistive tech while loading. */
  loadingLabel?: string;
  /** Render as the child element (e.g. Next.js Link) while keeping Button styles. */
  asChild?: boolean;
}

const base =
  "relative inline-flex h-10 items-center justify-center gap-2 px-4 " +
  "text-sm font-bold transition-[transform,background-color,border-color,box-shadow] duration-150 " +
  "hover:-translate-y-px disabled:pointer-events-none disabled:opacity-60 " +
  "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--ring)]";

const variants: Record<ButtonVariant, string> = {
  primary:
    "bg-gradient-to-r from-[var(--color-brand)] via-[color-mix(in_oklab,var(--color-brand)_85%,var(--color-gold))] to-[var(--color-brand)] " +
    "text-[var(--primary-foreground)] shadow-md shadow-[color-mix(in_oklab,var(--color-brand)_18%,transparent)] hover:brightness-105",
  secondary:
    "border border-[var(--border)] bg-[var(--surface)] text-[var(--foreground)] hover:bg-[var(--muted)] hover:border-[var(--color-gold)]/50",
  ghost: "text-[var(--foreground)] hover:bg-[var(--muted)]",
  gold:
    "bg-gradient-to-r from-[var(--color-gold)] to-[color-mix(in_oklab,var(--color-gold)_80%,#fff)] " +
    "text-[var(--color-navy)] shadow-md shadow-[color-mix(in_oklab,var(--color-gold)_18%,transparent)] hover:brightness-105",
  navy:
    "bg-gradient-to-r from-[var(--color-navy)] to-[var(--color-navy-soft)] " +
    "text-[var(--color-cream)] shadow-md shadow-[color-mix(in_oklab,var(--color-navy)_18%,transparent)] hover:brightness-105",
};

/** Wave-dot loader — three dots rise in sequence; width stays stable. */
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
    pill = false,
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
      className={cn(
        base,
        variants[variant],
        pill && "rounded-full",
        !pill && "rounded-[var(--radius-sm)]",
        variant === "primary" && "btn-shine",
        variant === "gold" && "btn-shine",
        variant === "navy" && "btn-shine",
        className,
      )}
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
})
