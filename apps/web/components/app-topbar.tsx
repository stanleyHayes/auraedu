"use client";

import { ThemeToggle, cn } from "@auraedu/ui";
import type { Tenant } from "@/lib/tenant";

interface AppTopbarProps {
  tenants: Tenant[];
  current: Tenant;
  onSelect: (t: Tenant) => void;
}

/** Portal top bar: breadcrumb, the tenant "preview as" switcher, theme toggle, avatar. */
export function AppTopbar({ tenants, current, onSelect }: AppTopbarProps) {
  return (
    <header className="flex h-[60px] items-center gap-3 border-b border-border bg-background/90 px-5 backdrop-blur">
      <span className="font-mono text-xs text-muted-foreground max-sm:hidden">
        Teaching&nbsp;/&nbsp;<b className="font-semibold text-foreground">Attendance</b>
      </span>
      <span className="flex-1" />
      <div className="hidden items-center gap-2 md:flex">
        <span className="font-mono text-[10.5px] uppercase tracking-[0.14em] text-muted-foreground">Preview as</span>
        {tenants.map((t) => {
          const isCurrent = t.code === current.code;
          return (
            <button
              key={t.code}
              type="button"
              onClick={() => onSelect(t)}
              aria-pressed={isCurrent}
              className={cn(
                "flex h-8 items-center gap-2 rounded-full border px-3 text-xs transition-colors",
                isCurrent
                  ? "border-[var(--primary)] text-foreground shadow-[inset_0_0_0_1px_var(--primary)]"
                  : "border-border text-muted-foreground hover:text-foreground",
              )}
            >
              <span className="size-3 rounded-full" style={{ backgroundColor: t.brand }} aria-hidden="true" />
              {t.short}
            </button>
          );
        })}
      </div>
      <ThemeToggle />
      <span
        className="grid size-9 place-items-center rounded-full border border-border bg-[var(--accent)] font-display text-sm font-extrabold text-[var(--primary)]"
        aria-label="Efua Mensah"
      >
        EM
      </span>
    </header>
  );
}
