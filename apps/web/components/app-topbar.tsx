"use client";

import { LogOut, Menu, User, Shield, Building2, CreditCard, Receipt, ScrollText, HeartPulse, Settings2, Users, GraduationCap, CalendarDays, ClipboardList } from "lucide-react";
import { ThemeToggle, UserMenu, type UserMenuItem, AdminDropdown, type AdminDropdownItem, NotificationBell, PillNav } from "@auraedu/ui";
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

const SUPERADMIN_LINKS: AdminDropdownItem[] = [
  { id: "tenants", label: "Tenants", description: "Manage schools", icon: <Building2 className="size-4" />, href: "/superadmin/tenants" },
  { id: "billing-plans", label: "Billing plans", description: "Pricing & plans", icon: <CreditCard className="size-4" />, href: "/superadmin/billing-plans" },
  { id: "subscriptions", label: "Subscriptions", description: "Active subscriptions", icon: <Receipt className="size-4" />, href: "/superadmin/subscriptions" },
  { id: "audit-logs", label: "Audit logs", description: "Platform activity", icon: <ScrollText className="size-4" />, href: "/superadmin/audit-logs" },
  { id: "system-health", label: "System health", description: "Service status", icon: <HeartPulse className="size-4" />, href: "/superadmin/system-health" },
];

const ADMIN_LINKS: AdminDropdownItem[] = [
  { id: "students", label: "Students", description: "Enrolment & records", icon: <Users className="size-4" />, href: "/admin/students" },
  { id: "staff", label: "Staff", description: "Teachers & employees", icon: <GraduationCap className="size-4" />, href: "/admin/staff" },
  { id: "academic-years", label: "Academic years", description: "Terms & calendars", icon: <CalendarDays className="size-4" />, href: "/admin/academic-years" },
  { id: "assessments", label: "Assessments", description: "Exams & grading", icon: <ClipboardList className="size-4" />, href: "/admin/assessments" },
  { id: "settings", label: "Settings", description: "School configuration", icon: <Settings2 className="size-4" />, href: "/admin/settings" },
];

export function AppTopbar({
  currentCode,
  onSelect,
  user,
  onLogout,
  onMobileMenuToggle,
  showMobileMenu = false,
}: AppTopbarProps) {
  const profileHref =
    user?.role === "teacher"
      ? "/teacher"
      : user?.role === "parent"
        ? "/parent"
        : user?.role === "student"
          ? "/student"
          : "/admin/settings";

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
      description: "Log out of the portal",
      icon: <LogOut className="size-4" />,
      onClick: onLogout,
      destructive: true,
    },
  ];

  const adminLinks = user?.role === "superadmin" ? SUPERADMIN_LINKS : user?.role === "admin" ? ADMIN_LINKS : [];

  const previewItems = SWITCHER.map((school) => ({
    id: school.code,
    label: (
      <>
        <span
          className="size-2.5 rounded-full"
          style={{ backgroundColor: school.swatch }}
          aria-hidden="true"
        />
        <span className="hidden sm:inline">{school.short}</span>
      </>
    ),
    active: school.code === currentCode,
    onClick: () => onSelect?.(school.code),
  }));

  return (
    <header className="topbar-glass sticky top-0 z-40 flex h-[60px] items-center gap-3 px-5">
      {showMobileMenu ? (
        <button
          type="button"
          onClick={onMobileMenuToggle}
          aria-label="Open navigation"
          className="grid size-10 place-items-center rounded-full border border-[var(--border)] bg-[var(--surface)] text-[var(--foreground)] md:hidden"
          data-tour="mobile-navigation"
        >
          <Menu className="size-5" />
        </button>
      ) : null}
      <span className="hidden items-center gap-2 font-mono text-xs text-muted-foreground sm:flex">
        <Shield className="size-3.5 text-[var(--color-gold)]" aria-hidden="true" />
        AuraEDU&nbsp;/&nbsp;<b className="font-semibold text-foreground">Portal</b>
      </span>
      <span className="flex-1" />
      {onSelect ? (
        <div className="hidden items-center gap-3 md:flex">
          <span className="font-mono text-[10.5px] uppercase tracking-[0.14em] text-muted-foreground">
            Preview as
          </span>
          <PillNav items={previewItems} />
        </div>
      ) : null}
      <span data-tour="theme-toggle">
        <ThemeToggle />
      </span>
      <NotificationBell count={0} />
      {adminLinks.length > 0 ? <AdminDropdown items={adminLinks} /> : null}
      <UserMenu user={user} items={menuItems} />
    </header>
  );
}
