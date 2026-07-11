"use client";

import * as React from "react";
import { cn } from "../lib/cn";

export interface UserMenuItem {
  id: string;
  label: string;
  description?: string;
  icon?: React.ReactNode;
  href?: string;
  onClick?: () => void;
  destructive?: boolean;
}

export interface UserMenuProps {
  user?: {
    name?: string;
    email?: string;
    role?: string;
    initials?: string;
  };
  items?: UserMenuItem[];
  align?: "start" | "end";
  className?: string;
}

function MenuRow({ item }: { item: UserMenuItem }) {
  const content = (
    <>
      {item.icon ? (
        <span className="mt-0.5 size-4 shrink-0 text-muted-foreground">{item.icon}</span>
      ) : null}
      <span className="flex min-w-0 flex-col">
        <span
          className={cn(
            "text-sm font-medium",
            item.destructive ? "text-[var(--color-crit)]" : "text-[var(--foreground)]",
          )}
        >
          {item.label}
        </span>
        {item.description ? (
          <span className="text-xs leading-4 text-[var(--muted-foreground)]">
            {item.description}
          </span>
        ) : null}
      </span>
    </>
  );

  const classes = cn(
    "flex w-full items-start gap-3 rounded-[var(--radius-sm)] p-3 text-left transition-colors",
    "hover:bg-[var(--muted)] focus-visible:bg-[var(--muted)] focus-visible:outline-none",
    item.destructive && "hover:bg-[color-mix(in_oklab,var(--color-crit)_8%,var(--muted))]",
  );

  if (item.href) {
    return (
      <a href={item.href} onClick={item.onClick} className={classes}>
        {content}
      </a>
    );
  }

  return (
    <button type="button" onClick={item.onClick} className={classes}>
      {content}
    </button>
  );
}

/** User avatar + dropdown with rich icon/title/description rows (DESIGN_SYSTEM §8). */
export function UserMenu({ user, items = [], align = "end", className }: UserMenuProps) {
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

  const initials =
    user?.initials ??
    (user?.name
      ? user.name
          .split(" ")
          .map((n) => n[0])
          .join("")
          .slice(0, 2)
          .toUpperCase()
      : "U");

  return (
    <div ref={ref} className={cn("relative", className)} data-tour="user-menu">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label="Account menu"
        className="grid size-9 place-items-center rounded-full border border-[var(--border)] bg-[var(--accent)] font-display text-sm font-extrabold text-[var(--primary)] transition-colors hover:brightness-95"
      >
        {initials}
      </button>
      {open ? (
        <div
          role="menu"
          className={cn(
            "absolute top-full z-[220] mt-2 w-72 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-2 shadow-lg",
            "motion-safe:animate-[slide-up_180ms_var(--ease-out-quart)]",
            align === "end" ? "right-0" : "left-0",
          )}
        >
          {user ? (
            <div className="px-3 py-2">
              <p className="truncate text-sm font-semibold text-[var(--foreground)]">
                {user.name ?? "Account"}
              </p>
              {user.email ? (
                <p className="truncate text-xs text-[var(--muted-foreground)]">{user.email}</p>
              ) : null}
              {user.role ? (
                <p className="mt-1 font-mono text-[10px] uppercase tracking-[0.14em] text-[var(--muted-foreground)]">
                  {user.role}
                </p>
              ) : null}
            </div>
          ) : null}
          {user && items.length > 0 ? <div className="my-1 h-px bg-[var(--border)]" /> : null}
          <div className="space-y-0.5">
            {items.map((item) => (
              <div key={item.id} role="menuitem">
                <MenuRow item={item} />
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}
