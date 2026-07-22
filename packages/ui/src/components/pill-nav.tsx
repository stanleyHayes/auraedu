"use client";

import * as React from "react";
import { cn } from "../lib/cn";

export interface PillNavItem {
  id: string;
  label: React.ReactNode;
  active?: boolean;
  href?: string;
  onClick?: () => void;
  disabled?: boolean;
}

export interface PillNavProps {
  items: PillNavItem[];
  className?: string;
  /** Accent colour for the sliding indicator; defaults to primary. */
  indicatorClassName?: string;
}

/** Horizontal pill navigation with a sliding active indicator. */
export function PillNav({ items, className, indicatorClassName }: PillNavProps) {
  const containerRef = React.useRef<HTMLDivElement>(null);
  const [indicator, setIndicator] = React.useState({ left: 0, width: 0 });

  React.useLayoutEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const active = container.querySelector<HTMLElement>("[data-pill-active='true']");
    if (active) {
      setIndicator({ left: active.offsetLeft, width: active.offsetWidth });
    } else {
      setIndicator({ left: 0, width: 0 });
    }
  }, [items]);

  return (
    <div
      ref={containerRef}
      role="tablist"
      className={cn("pill-nav relative isolate inline-flex items-center", className)}
    >
      {indicator.width > 0 ? (
        <span
          className={cn(
            "pointer-events-none absolute top-1 bottom-1 rounded-full bg-[var(--primary)] shadow-sm transition-all duration-300",
            indicatorClassName,
          )}
          style={{ left: indicator.left, width: indicator.width }}
          aria-hidden="true"
        />
      ) : null}
      {items.map((item) => {
        const content = (
          <span
            data-pill-active={item.active ? "true" : "false"}
            role={item.href ? undefined : "tab"}
            aria-selected={item.active}
            aria-disabled={item.disabled}
            className={cn(
              "pill-nav__item relative z-10",
              item.active && "text-[var(--primary-foreground)]",
              item.disabled && "cursor-not-allowed opacity-50",
            )}
          >
            {item.label}
          </span>
        );
        if (item.href) {
          return (
            <a
              key={item.id}
              href={item.href}
              onClick={item.onClick}
              className="contents"
              aria-current={item.active ? "page" : undefined}
            >
              {content}
            </a>
          );
        }
        return (
          <button
            key={item.id}
            type="button"
            role="tab"
            disabled={item.disabled}
            onClick={item.onClick}
            className="contents"
          >
            {content}
          </button>
        );
      })}
    </div>
  );
}
