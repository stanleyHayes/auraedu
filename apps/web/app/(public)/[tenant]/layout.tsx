import Link from "next/link";
import type { TenantData } from "@/lib/tenant";
import { fetchTenantBranding } from "@/lib/tenant";
import { fetchWebsitePages, type WebsitePage } from "@/lib/website";

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

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground" style={themeStyle}>
      <PublicHeader tenant={tenant} tenantCode={tenantCode} pages={navPages} />
      <main className="flex-1">{children}</main>
      <PublicFooter tenant={tenant} />
    </div>
  );
}

function PublicHeader({
  tenant,
  tenantCode,
  pages,
}: {
  tenant: TenantData;
  tenantCode: string;
  pages: WebsitePage[];
}) {
  const homeHref = `/${tenantCode}`;

  return (
    <header className="sticky top-0 z-30 border-b border-[var(--border)] bg-background/95 backdrop-blur-sm">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
        <Link href={homeHref} className="flex items-center gap-3">
          {tenant.branding.logo_url ? (
            <img
              src={tenant.branding.logo_url}
              alt=""
              className="size-10 rounded-[var(--radius-sm)] object-contain"
            />
          ) : (
            <span className="grid size-10 place-items-center rounded-[var(--radius-sm)] bg-[var(--primary)] font-display text-lg font-extrabold text-[var(--primary-foreground)]">
              {tenant.short.charAt(0)}
            </span>
          )}
          <span className="font-display text-xl font-bold text-[var(--foreground)]">{tenant.name}</span>
        </Link>

        {pages.length > 0 ? (
          <nav aria-label="School website">
            <ul className="hidden items-center gap-1 md:flex">
              {pages.map((page) => (
                <li key={page.id}>
                  <PublicNavLink tenantCode={tenantCode} page={page} />
                </li>
              ))}
            </ul>
          </nav>
        ) : null}
      </div>

      {pages.length > 0 ? (
        <nav aria-label="School website mobile" className="border-t border-[var(--border)] md:hidden">
          <ul className="flex items-center gap-4 overflow-x-auto px-6 py-3">
            {pages.map((page) => (
              <li key={page.id} className="shrink-0">
                <PublicNavLink tenantCode={tenantCode} page={page} />
              </li>
            ))}
          </ul>
        </nav>
      ) : null}
    </header>
  );
}

function PublicNavLink({ tenantCode, page }: { tenantCode: string; page: WebsitePage }) {
  const href = page.slug === "home" ? `/${tenantCode}` : `/${tenantCode}/${page.slug}`;
  return (
    <Link
      href={href}
      className="rounded-[var(--radius-sm)] px-3 py-2 text-sm font-medium text-[var(--muted-foreground)] transition-colors hover:bg-[var(--muted)] hover:text-[var(--foreground)]"
    >
      {page.title}
    </Link>
  );
}

function PublicFooter({ tenant }: { tenant: TenantData }) {
  return (
    <footer className="border-t border-[var(--border)] bg-[var(--surface)]">
      <div className="mx-auto max-w-6xl px-6 py-8">
        <div className="flex flex-col items-center justify-between gap-4 md:flex-row">
          <div className="flex items-center gap-2">
            <span className="font-display text-lg font-bold text-[var(--foreground)]">{tenant.name}</span>
          </div>
          <p className="text-sm text-[var(--muted-foreground)]">
            © {new Date().getFullYear()} {tenant.name}. Powered by AuraEDU.
          </p>
        </div>
      </div>
    </footer>
  );
}
