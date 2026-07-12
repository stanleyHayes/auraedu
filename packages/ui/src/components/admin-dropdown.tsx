"use client";

import * as React from "react";
import { ChevronDown, Layers } from "lucide-react";
import { cn } from "../lib/cn";

export interface AdminDropdownItem {
  id: string;
  label: string;
  description?: string;
  icon?: React.ReactNode;
  href?: string;
  onClick?: () => void;
}

export interface AdminDropdownProps {
  label?: string;
  items: AdminDropdownItem[];
  className?: string;
  align?: "start" | "end";
}

function MenuRow({ item }: { item: AdminDropdownItem }) {
  const content = (
    <>
      {item.icon ? (
        <span className="mt-0.5 size-4 shrink-0 text-[var(--color-gold)]">{item.icon}</span>
      ) : null}
      <span className="flex min-w-0 flex-col">
        <span className="text-sm font-semibold text-[var(--foreground)]">{item.label}</span>
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

/** Admin quick-links dropdown with a mega-menu style panel. */
export function AdminDropdown({
  label = "Admin",
  items,
  className,
  align = "end",
}: AdminDropdownProps) {
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
        className={cn(
          "inline-flex h-10 items-center gap-2 rounded-full border border-[var(--border)] bg-[var(--surface)] px-3.5 text-sm font-semibold text-[var(--foreground)]",
          "transition-colors hover:bg-[var(--muted)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]",
          open && "bg-[var(--muted)]",
        )}
      >
        <Layers className="size-4 text-[var(--color-gold)]" aria-hidden="true" />
        <span className="hidden sm:inline">{label}</span>
        <ChevronDown
          className={cn("size-4 text-[var(--muted-foreground)] transition-transform", open && "rotate-180")}
          aria-hidden="true"
        />
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
          <div className="border-b border-[var(--border)] px-3 py-2">
            <p className="font-heading text-sm font-bold text-[var(--foreground)]">Admin tools</p>
            <p className="text-xs text-[var(--muted-foreground)]">Jump to management areas</p>
          </div>
          <div className="mt-1 space-y-0.5">
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
