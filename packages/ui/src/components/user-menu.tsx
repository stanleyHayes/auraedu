"use client";

import * as React from "react";
import { User, ChevronRight } from "lucide-react";
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
        <span
          className={cn(
            "mt-0.5 size-4 shrink-0",
            item.destructive ? "text-[var(--color-crit)]" : "text-[var(--color-gold)]",
          )}
        >
          {item.icon}
        </span>
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
      <ChevronRight className="ml-auto size-4 shrink-0 text-[var(--muted-foreground)] opacity-0 transition-opacity group-hover:opacity-100" />
    </>
  );

  const classes = cn(
    "group flex w-full items-start gap-3 rounded-[var(--radius-sm)] p-3 text-left transition-colors",
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

/** User avatar + mega-menu dropdown with rich icon/title/description rows. */
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
        className={cn(
          "grid size-10 place-items-center overflow-hidden rounded-full border-2 border-[var(--color-gold)]/30 bg-gradient-to-br from-[var(--color-brand)] to-[var(--color-burgundy)]",
          "font-sans text-sm font-extrabold text-white shadow-sm transition-transform hover:scale-105",
        )}
      >
        {initials}
      </button>
      {open ? (
        <div
          role="menu"
          className={cn(
            "absolute top-full z-[220] mt-2 w-80 rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface)] p-2 shadow-xl",
            "motion-safe:animate-[slide-up_180ms_var(--ease-out-quart)]",
            align === "end" ? "right-0" : "left-0",
          )}
        >
          {user ? (
            <div className="flex items-start gap-3 px-3 py-3">
              <span className="grid size-10 shrink-0 place-items-center rounded-full bg-gradient-to-br from-[var(--color-brand)] to-[var(--color-burgundy)] font-sans text-sm font-extrabold text-white">
                {initials}
              </span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-semibold text-[var(--foreground)]">
                  {user.name ?? "Account"}
                </p>
                {user.email ? (
                  <p className="truncate text-xs text-[var(--muted-foreground)]">{user.email}</p>
                ) : null}
                {user.role ? (
                  <p className="mt-1 inline-block rounded-full bg-[var(--color-gold)]/10 px-2 py-0.5 font-mono text-[10px] uppercase tracking-[0.12em] text-[var(--color-gold)]">
                    {user.role}
                  </p>
                ) : null}
              </div>
            </div>
          ) : null}
          {user && items.length > 0 ? <div className="my-1 h-px bg-[var(--border)]" /> : null}
          <div className="space-y-0.5">
            {items.length === 0 ? (
              <div className="px-3 py-2 text-sm text-[var(--muted-foreground)]">
                <User className="mb-1 size-4" />
                Account options will appear here.
              </div>
            ) : (
              items.map((item) => (
                <div key={item.id} role="menuitem">
                  <MenuRow item={item} />
                </div>
              ))
            )}
          </div>
        </div>
      ) : null}
    </div>
  );
}
