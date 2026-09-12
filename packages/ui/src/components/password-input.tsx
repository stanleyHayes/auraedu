"use client";

import * as React from "react";
import { Eye, EyeOff } from "lucide-react";
import { cn } from "../lib/cn";
import { Input, type InputProps } from "./input";

export interface PasswordInputProps extends Omit<InputProps, "type"> {
  showLabel?: string;
  hideLabel?: string;
}

/** Password field with a keyboard-accessible, non-submitting visibility control. */
export const PasswordInput = React.forwardRef<HTMLInputElement, PasswordInputProps>(
  function PasswordInput(
    { className, showLabel = "Show password", hideLabel = "Hide password", disabled, ...props },
    ref,
  ) {
    const [visible, setVisible] = React.useState(false);

    return (
      <span className="password-field relative block">
        <Input
          ref={ref}
          type={visible ? "text" : "password"}
          disabled={disabled}
          className={cn("pr-12", className)}
          {...props}
        />
        <button
          type="button"
          aria-label={visible ? hideLabel : showLabel}
          aria-pressed={visible}
          disabled={disabled}
          onClick={() => setVisible((current) => !current)}
          className="password-toggle absolute inset-y-1 right-1 grid aspect-square place-items-center rounded-[calc(var(--radius-md)-0.2rem)] text-[var(--muted-foreground)] transition-[color,background-color,transform] hover:bg-[var(--muted)] hover:text-[var(--foreground)] active:scale-95 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-[var(--ring)] disabled:pointer-events-none disabled:opacity-50"
        >
          {visible ? (
            <EyeOff className="size-4.5" aria-hidden="true" />
          ) : (
            <Eye className="size-4.5" aria-hidden="true" />
          )}
        </button>
      </span>
    );
  },
);
