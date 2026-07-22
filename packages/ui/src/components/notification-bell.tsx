"use client";

import * as React from "react";
import { Bell, BellOff } from "lucide-react";
import { cn } from "../lib/cn";

export interface NotificationBellProps {
  count?: number;
  className?: string;
  align?: "start" | "end";
}

/** Notification bell with a dropdown panel and unread badge. */
export function NotificationBell({ count = 0, className, align = "end" }: NotificationBellProps) {
  const [open, setOpen] = React.useState(false);
  const ref = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    if (!open) return;
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    function onClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("keydown", onKeyDown);
    document.addEventListener("mousedown", onClickOutside);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.removeEventListener("mousedown", onClickOutside);
    };
  }, [open]);

  return (
    <div ref={ref} className={cn("relative", className)}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Notifications${count > 0 ? `, ${count} unread` : ""}`}
        className={cn(
          "relative grid size-10 place-items-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--surface)] text-[var(--foreground)]",
          "transition-colors hover:bg-[var(--muted)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]",
        )}
      >
        <Bell className="size-[18px]" aria-hidden="true" />
        {count > 0 ? (
          <span className="absolute right-1.5 top-1.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-[var(--color-crit)] px-1 text-[10px] font-bold text-white">
            {count > 9 ? "9+" : count}
          </span>
        ) : null}
      </button>
      {open ? (
        <div
          role="menu"
          className={cn(
            "absolute top-full z-[220] mt-2 w-80 rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface)] p-4 shadow-xl",
            "motion-safe:animate-[slide-up_180ms_var(--ease-out-quart)]",
            align === "end" ? "right-0" : "left-0",
          )}
        >
          <div className="flex items-center justify-between border-b border-[var(--border)] pb-3">
            <p className="font-heading text-sm font-bold text-[var(--foreground)]">Notifications</p>
            {count > 0 ? (
              <span className="rounded-full bg-[var(--color-signal)]/12 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-[var(--color-forest)]">
                {count} new
              </span>
            ) : null}
          </div>
          <div className="flex flex-col items-center justify-center py-8 text-center">
            <BellOff className="size-8 text-[var(--muted-foreground)]" aria-hidden="true" />
            <p className="mt-3 text-sm font-medium text-[var(--foreground)]">
              No notifications yet
            </p>
            <p className="mt-1 text-xs text-[var(--muted-foreground)]">
              Alerts and updates will appear here.
            </p>
          </div>
        </div>
      ) : null}
    </div>
  );
}
