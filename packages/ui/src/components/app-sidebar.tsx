"use client";

import * as React from "react";
import { cn } from "../lib/cn";

export interface NavItem {
  label: string;
  href: string;
  badge?: number;
}
export interface NavGroup {
  heading: string;
  items: NavItem[];
}
export interface AppSidebarProps {
  /** Brand block (school name + mark). */
  brand: React.ReactNode;
  groups: NavGroup[];
  /** Current path — the app passes `usePathname()` (keeps this component framework-agnostic). */
  pathname: string;
  onNavigate?: (href: string) => void;
  className?: string;
}

const STORAGE_KEY = "auraedu-nav-groups";

function isActive(pathname: string, href: string): boolean {
  return pathname === href || pathname.startsWith(`${href}/`);
}

function Tick({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 12" className={className} aria-hidden="true">
      <path
        d="M1 6.5 5.2 10.5 15 1"
        fill="none"
        stroke="currentColor"
        strokeWidth={2.4}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

/** Curved file-tree connector from the group trunk into each item. */
function Connector({ last, active }: { last: boolean; active: boolean }) {
  return (
    <svg
      viewBox="0 0 24 40"
      preserveAspectRatio="none"
      aria-hidden="true"
      className={cn(
        "absolute left-[18px] top-0 h-full w-5 transition-colors",
        active ? "text-[var(--color-gold)]" : "text-[var(--color-navy-muted)]",
      )}
    >
      <path
        d={last ? "M7 0 V17 Q7 23 13 23 H24" : "M7 0 V40 M7 23 Q7 23 13 23 H24"}
        fill="none"
        stroke="currentColor"
        strokeWidth={1.5}
        strokeLinecap="round"
      />
    </svg>
  );
}

/**
 * Dark navy command-center sidebar with gold accents, curved connectors,
 * and collapsible groups. Inspired by NADAA / UPOSA / xtiitch dashboard rails.
 */
export function AppSidebar({ brand, groups, pathname, onNavigate, className }: AppSidebarProps) {
  const [openState, setOpenState] = React.useState<Record<string, boolean>>({});

  React.useEffect(() => {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (raw) setOpenState(JSON.parse(raw) as Record<string, boolean>);
    } catch {
      /* ignore */
    }
  }, []);

  const toggle = React.useCallback((heading: string) => {
    setOpenState((cur) => {
      const next = { ...cur, [heading]: !(cur[heading] ?? true) };
      try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
      } catch {
        /* ignore */
      }
      return next;
    });
  }, []);

  return (
    <aside
      className={cn(
        "relative flex w-60 flex-col overflow-y-auto border-r border-[var(--color-navy-soft)]",
        "bg-[var(--color-navy)] text-[var(--color-cream)]",
        className,
      )}
      aria-label="Primary"
    >
      {/* subtle watermark */}
      <span
        aria-hidden="true"
        className="pointer-events-none absolute -right-10 top-20 select-none font-heading text-[9rem] font-extrabold leading-none tracking-tighter text-white/[0.03]"
      >
        A
      </span>

      <div className="relative z-10 px-4 pb-3 pt-4">{brand}</div>
      <nav className="relative z-10 pb-4">
        {groups.map((group) => {
          const hasActive = group.items.some((i) => isActive(pathname, i.href));
          const isOpen = hasActive || (openState[group.heading] ?? true);
          const panelId = `nav-${group.heading.replace(/\s+/g, "-").toLowerCase()}`;
          return (
            <div key={group.heading} className="py-1">
              <button
                type="button"
                onClick={() => toggle(group.heading)}
                aria-expanded={isOpen}
                aria-controls={panelId}
                className="flex w-full items-center justify-between px-[18px] py-1.5 font-mono text-[10.5px] uppercase tracking-[0.14em] text-[var(--color-gold-muted)] transition-colors hover:text-[var(--color-gold-soft)]"
              >
                {group.heading}
                <svg
                  viewBox="0 0 24 24"
                  className={cn("size-3 transition-transform", !isOpen && "-rotate-90")}
                  fill="none"
                  stroke="currentColor"
                  strokeWidth={2.4}
                  strokeLinecap="round"
                >
                  <path d="M6 9l6 6 6-6" />
                </svg>
              </button>
              <div
                id={panelId}
                className={cn(
                  "grid transition-[grid-template-rows,opacity] duration-200 ease-out",
                  isOpen ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0",
                )}
              >
                <div className="min-h-0 overflow-hidden">
                  {group.items.map((item, i) => {
                    const active = isActive(pathname, item.href);
                    return (
                      <a
                        key={item.href}
                        href={item.href}
                        onClick={() => onNavigate?.(item.href)}
                        aria-current={active ? "page" : undefined}
                        className={cn(
                          "relative flex h-[38px] items-center gap-2 pl-10 pr-3.5 text-[13.5px] transition-colors",
                          active
                            ? "font-semibold text-[var(--color-gold)]"
                            : "text-[var(--color-cream)]/70 hover:text-[var(--color-cream)]",
                        )}
                      >
                        <Connector last={i === group.items.length - 1} active={active} />
                        {active ? (
                          <span
                            aria-hidden="true"
                            className="absolute bottom-2 left-0 top-2 w-[3px] rounded-r bg-[var(--color-gold)]"
                          />
                        ) : null}
                        <span className="truncate">{item.label}</span>
                        {item.badge ? (
                          <span className="ml-auto rounded-full bg-[var(--color-crit)] px-1.5 font-mono text-[10px] text-white">
                            {item.badge}
                          </span>
                        ) : active ? (
                          <Tick className="ml-auto w-3.5 text-[var(--color-gold)]" />
                        ) : null}
                      </a>
                    );
                  })}
                </div>
              </div>
            </div>
          );
        })}
      </nav>
    </aside>
  );
}
