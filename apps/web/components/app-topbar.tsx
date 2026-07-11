"use client";

import { LogOut, Menu, User } from "lucide-react";
import { ThemeToggle, UserMenu, type UserMenuItem, cn } from "@auraedu/ui";
import { SWITCHER } from "@/lib/tenant";

interface AppTopbarProps {
  currentCode?: string;
  onSelect?: (code: string) => void;
  user?: {
    name?: string;
    email?: string;
    role?: string;
    initials?: string;
  };
  onLogout?: () => void;
  onMobileMenuToggle?: () => void;
  showMobileMenu?: boolean;
}

export function AppTopbar({
  currentCode,
  onSelect,
  user,
  onLogout,
  onMobileMenuToggle,
  showMobileMenu = false,
}: AppTopbarProps) {
  const profileHref =
    user?.role === "teacher" ? "/teacher" : user?.role === "parent" ? "/parent" : "/admin/settings";

  const menuItems: UserMenuItem[] = [
    {
      id: "profile",
      label: "Profile",
      description: "View your account details",
      icon: <User className="size-4" />,
      href: profileHref,
    },
    {
      id: "logout",
      label: "Sign out",
      description: "Log out of the admin console",
      icon: <LogOut className="size-4" />,
      onClick: onLogout,
      destructive: true,
    },
  ];

  return (
    <header className="flex h-[60px] items-center gap-3 border-b border-border bg-background/90 px-5 backdrop-blur">
      {showMobileMenu ? (
        <button
          type="button"
          onClick={onMobileMenuToggle}
          aria-label="Open navigation"
          className="grid size-10 place-items-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--surface)] text-[var(--foreground)] md:hidden"
          data-tour="mobile-navigation"
        >
          <Menu className="size-5" />
        </button>
      ) : null}
      <span className="font-mono text-xs text-muted-foreground max-sm:hidden">
        AuraEDU&nbsp;/&nbsp;<b className="font-semibold text-foreground">Portal</b>
      </span>
      <span className="flex-1" />
      {onSelect ? (
        <div className="hidden items-center gap-2 md:flex">
          <span className="font-mono text-[10.5px] uppercase tracking-[0.14em] text-muted-foreground">Preview as</span>
          {SWITCHER.map((school) => {
            const isCurrent = school.code === currentCode;
            return (
              <button
                key={school.code}
                type="button"
                onClick={() => onSelect(school.code)}
                aria-pressed={isCurrent}
                className={cn(
                  "flex h-8 items-center gap-2 rounded-full border px-3 text-xs transition-colors",
                  isCurrent
                    ? "border-[var(--primary)] text-foreground shadow-[inset_0_0_0_1px_var(--primary)]"
                    : "border-border text-muted-foreground hover:text-foreground",
                )}
              >
                <span className="size-3 rounded-full" style={{ backgroundColor: school.swatch }} aria-hidden="true" />
                {school.short}
              </button>
            );
          })}
        </div>
      ) : null}
      <span data-tour="theme-toggle">
        <ThemeToggle />
      </span>
      <UserMenu user={user} items={menuItems} />
    </header>
  );
}
