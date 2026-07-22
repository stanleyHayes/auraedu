import Link from "next/link";
import { ArrowRight } from "lucide-react";
import type { TenantData } from "@/lib/tenant";
import { fetchTenantBranding } from "@/lib/tenant";
import { fetchWebsitePages, type WebsitePage } from "@/lib/website";
import { PillNav, Watermark } from "@auraedu/ui";
import { AdmissionsAssistant } from "@/components/admissions-assistant";
import { AuraEduLogo } from "@/components/auraedu-logo";
import { isFeatureEnabled } from "@/lib/features";

interface PublicLayoutProps {
  children: React.ReactNode;
  params: Promise<{ tenant: string }>;
}

export default async function PublicLayout({ children, params }: PublicLayoutProps) {
  const { tenant: tenantCode } = await params;

  const [tenant, pages] = await Promise.all([
    fetchTenantBranding(tenantCode),
    fetchWebsitePages(tenantCode, { status: "published" }),
  ]);

  const navPages = pages.filter((page) => page.status === "published").slice(0, 6);

  return (
    <PublicShell tenant={tenant} tenantCode={tenantCode} navPages={navPages}>
      {children}
    </PublicShell>
  );
}

function PublicShell({
  tenant,
  tenantCode,
  navPages,
  children,
}: {
  tenant: TenantData;
  tenantCode: string;
  navPages: WebsitePage[];
  children: React.ReactNode;
}) {
  const primary = tenant.branding.brand.primary;
  const secondary = tenant.branding.brand.secondary;

  const themeStyle = {
    "--color-brand": primary,
    "--color-brand-deep": `color-mix(in oklab, ${primary} 80%, black)`,
    "--color-brand-tint": `color-mix(in oklab, ${primary} 14%, var(--color-paper-50))`,
    "--color-brand-contrast": "#FFFFFF",
    "--color-brand-secondary": secondary ?? primary,
  } as React.CSSProperties;

  const homeHref = `/${tenantCode}`;
  const navItems = navPages.map((page) => ({
    id: page.id,
    label: page.title,
    href: page.slug === "home" ? homeHref : `/${tenantCode}/${page.slug}`,
  }));
  if (isFeatureEnabled(tenant.features, "admissions")) {
    navItems.push({
      id: "programme-catalogue",
      label: "Programmes",
      href: `${homeHref}/programmes`,
    });
  }

  return (
    <div
      className="school-site relative flex min-h-screen flex-col bg-background text-foreground"
      style={themeStyle}
    >
      <a className="app-skip-link" href="#school-site-main">
        Skip to school content
      </a>
      <PublicHeader tenant={tenant} tenantCode={tenantCode} navItems={navItems} />
      <main id="school-site-main" className="relative flex-1 overflow-hidden" tabIndex={-1}>
        <Watermark className="pointer-events-none absolute left-1/2 top-24 -translate-x-1/2 text-[14rem] opacity-[0.025]">
          {tenant.short}
        </Watermark>
        {children}
      </main>
      {isFeatureEnabled(tenant.features, "growth_website_chat") ? (
        <AdmissionsAssistant tenantCode={tenantCode} schoolName={tenant.name} />
      ) : null}
      <PublicFooter tenant={tenant} />
    </div>
  );
}

function PublicHeader({
  tenant,
  tenantCode,
  navItems,
}: {
  tenant: TenantData;
  tenantCode: string;
  navItems: { id: string; label: string; href: string }[];
}) {
  const homeHref = `/${tenantCode}`;

  return (
    <header className="school-site-header sticky top-0 z-30 border-b border-[var(--border)] bg-background/88 backdrop-blur-xl">
      <div className="mx-auto flex max-w-7xl items-center justify-between gap-6 px-5 py-3.5 sm:px-6">
        <Link href={homeHref} className="flex min-w-0 items-center gap-3">
          {tenant.branding.logo_url ? (
            <img
              src={tenant.branding.logo_url}
              alt=""
              className="size-11 rounded-xl border border-[var(--border)] bg-[var(--surface)] object-contain p-1.5 shadow-sm"
            />
          ) : (
            <span className="grid size-11 place-items-center rounded-xl bg-gradient-to-br from-[var(--primary)] to-[var(--color-navy)] font-sans text-lg font-extrabold text-[var(--primary-foreground)] shadow-md">
              {tenant.short.charAt(0)}
            </span>
          )}
          <span className="min-w-0">
            <span className="block truncate font-sans text-base font-extrabold tracking-tight text-[var(--foreground)] sm:text-lg">
              {tenant.name}
            </span>
            <span className="block font-mono text-[8px] font-black uppercase tracking-[0.18em] text-[var(--muted-foreground)] sm:text-[9px]">
              Learning · community · progress
            </span>
          </span>
        </Link>

        <Link
          href="/login"
          className="inline-flex h-10 shrink-0 items-center gap-1.5 rounded-full bg-[var(--color-navy)] px-3.5 text-xs font-extrabold text-white shadow-md transition-transform hover:-translate-y-px md:hidden"
        >
          Portal <ArrowRight className="size-3.5" aria-hidden="true" />
        </Link>

        <div className="hidden items-center gap-3 md:flex">
          {navItems.length > 0 ? (
            <nav aria-label="School website">
              <PillNav items={navItems} />
            </nav>
          ) : null}
          <Link
            href="/login"
            className="inline-flex h-10 items-center gap-2 rounded-full bg-[var(--color-navy)] px-4 text-sm font-extrabold text-white shadow-md transition-transform hover:-translate-y-px"
          >
            Portal <ArrowRight className="size-4" aria-hidden="true" />
          </Link>
        </div>
      </div>

      {navItems.length > 0 ? (
        <nav
          aria-label="School website mobile"
          className="border-t border-[var(--border)] bg-[var(--surface)]/76 md:hidden"
        >
          <ul className="flex items-center gap-2 overflow-x-auto px-5 py-3">
            {navItems.map((page) => (
              <li key={page.id} className="shrink-0">
                <a
                  href={page.href}
                  className="rounded-full border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-xs font-bold text-[var(--muted-foreground)] transition-colors hover:border-[var(--primary)] hover:text-[var(--foreground)]"
                >
                  {page.label}
                </a>
              </li>
            ))}
          </ul>
        </nav>
      ) : null}
    </header>
  );
}

function PublicFooter({ tenant }: { tenant: TenantData }) {
  return (
    <footer className="border-t border-white/10 bg-[var(--color-navy)] text-white">
      <div className="mx-auto max-w-7xl px-6 py-10">
        <div className="flex flex-col items-start justify-between gap-6 md:flex-row md:items-end">
          <div>
            <span className="font-sans text-xl font-extrabold tracking-tight">{tenant.name}</span>
            <p className="mt-2 max-w-md text-sm leading-6 text-white/58">
              A focused digital home for admissions, learning and the life of the school.
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-x-2 gap-y-2 text-sm text-white/58 md:justify-end">
            <span>
              © {new Date().getFullYear()} {tenant.name}.
            </span>
            <span className="text-white/35">Powered by</span>
            <AuraEduLogo
              tone="light"
              className="h-5 w-auto opacity-80 transition-opacity hover:opacity-100"
            />
          </div>
        </div>
      </div>
    </footer>
  );
}
