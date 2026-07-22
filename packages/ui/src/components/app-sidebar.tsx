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
  /** Short context label shown above the navigation groups. */
  workspaceLabel?: string;
  /** Optional compact footer content such as environment or support status. */
  footer?: React.ReactNode;
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
        active
          ? "text-[var(--portal-signal,var(--color-signal))]"
          : "text-[var(--color-navy-muted)]",
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
 * Dark navy command-center sidebar with signal accents, curved connectors,
 * and collapsible groups. Inspired by NADAA / UPOSA / xtiitch dashboard rails.
 */
export function AppSidebar({
  brand,
  groups,
  pathname,
  onNavigate,
  workspaceLabel = "Education workspace",
  footer,
  className,
}: AppSidebarProps) {
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
        "portal-sidebar relative flex w-72 flex-col overflow-y-auto border-r border-white/10",
        "bg-[var(--color-navy)] text-[var(--color-cream)]",
        className,
      )}
      aria-label="Primary"
    >
      {/* subtle watermark */}
      <span
        aria-hidden="true"
        className="pointer-events-none absolute -right-12 top-24 select-none font-heading text-[11rem] font-extrabold leading-none tracking-tighter text-white/[0.035]"
      >
        A
      </span>

      {brand ? <div className="relative z-10 px-5 pb-4 pt-5">{brand}</div> : null}
      <div className="relative z-10 mx-4 mb-3 overflow-hidden rounded-2xl border border-white/10 bg-white/[0.055] px-4 py-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]">
        <span className="absolute -right-4 -top-5 size-16 rounded-full bg-[var(--color-brand)]/25 blur-2xl" />
        <p className="relative font-mono text-[9px] font-bold uppercase tracking-[0.18em] text-[var(--portal-signal,var(--color-signal))]">
          Live workspace
        </p>
        <p className="relative mt-1 truncate text-[13px] font-semibold text-white/90">
          {workspaceLabel}
        </p>
      </div>
      <nav className="relative z-10 flex-1 pb-4">
        {groups.map((group) => {
          const hasActive = group.items.some((i) => isActive(pathname, i.href));
          const isOpen = hasActive || (openState[group.heading] ?? true);
          const panelId = `nav-${group.heading.replace(/\s+/g, "-").toLowerCase()}`;
          return (
            <div key={group.heading} className="px-3 py-1">
              <button
                type="button"
                onClick={() => toggle(group.heading)}
                aria-expanded={isOpen}
                aria-controls={panelId}
                className="flex w-full items-center justify-between px-2.5 py-2 font-mono text-[9.5px] font-bold uppercase tracking-[0.18em] text-white/45 transition-colors hover:text-[var(--portal-accent-soft,var(--color-teal-bright))]"
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
                          "group relative flex min-h-10 items-center gap-2 rounded-xl py-2 pl-10 pr-3 text-[13.5px] transition-[color,background-color,transform] duration-200",
                          active
                            ? "bg-white/[0.09] font-bold text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]"
                            : "text-[var(--color-cream)]/65 hover:translate-x-0.5 hover:bg-white/[0.045] hover:text-white",
                        )}
                      >
                        <Connector last={i === group.items.length - 1} active={active} />
                        {active ? (
                          <span
                            aria-hidden="true"
                            className="absolute bottom-2 left-0 top-2 w-[3px] rounded-full bg-gradient-to-b from-[var(--portal-signal,var(--color-signal))] to-[var(--portal-accent,var(--color-brand))] shadow-[0_0_14px_color-mix(in_oklab,var(--portal-accent,var(--color-brand))_45%,transparent)]"
                          />
                        ) : null}
                        <span className="truncate">{item.label}</span>
                        {item.badge ? (
                          <span className="ml-auto rounded-full bg-[var(--portal-signal,var(--color-signal))] px-2 py-0.5 font-mono text-[9px] font-black text-[var(--color-navy)]">
                            {item.badge}
                          </span>
                        ) : active ? (
                          <Tick className="ml-auto w-3.5 text-[var(--portal-signal,var(--color-signal))]" />
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
      {footer ? (
        <div className="relative z-10 m-4 mt-0 border-t border-white/10 pt-4">{footer}</div>
      ) : null}
    </aside>
  );
}
