import type { Metadata } from "next";
import { headers } from "next/headers";
import { notFound } from "next/navigation";
import { FeatureFlagsProvider, type FeatureSnapshot } from "@auraedu/flags";
import {
  DEFAULT_BRAND,
  brandContrastColor,
  brandOnDarkColor,
  getTenantCodeFromHeaders,
  isTenantNotFound,
  fetchTenantBranding,
  toFeatureSnapshot,
} from "@/lib/tenant";
import "./globals.css";

export const metadata: Metadata = {
  title: "AuraEDU — School portal",
  description: "The AuraEDU school portal.",
  robots: { index: false },
  icons: {
    icon: [
      { url: "/icon.svg", type: "image/svg+xml" },
      { url: "/favicon.ico", sizes: "any" },
    ],
    shortcut: "/icon.svg",
    apple: "/apple-icon.png",
  },
};

const bootScript = `(function(){try{var r=document.documentElement;var m=localStorage.getItem('auraedu-theme')||(matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light');r.classList.toggle('dark',m==='dark');r.style.colorScheme=m;}catch(e){}})();`;

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const requestHeaders = await headers();
  const tenantCode = getTenantCodeFromHeaders(requestHeaders);
  const pathname = requestHeaders.get("x-pathname") ?? "";
  const globalAuthEntry = isGlobalAuthEntry(pathname) && !tenantCode;

  if ((!tenantCode || isTenantNotFound(requestHeaders)) && !globalAuthEntry) {
    notFound();
  }

  let snapshot: FeatureSnapshot;
  let brand: string = DEFAULT_BRAND;
  let secondary: string | undefined;

  if (globalAuthEntry) {
    snapshot = { tenantCode: "auraedu", flags: [] };
  } else {
    let tenant;
    try {
      tenant = await fetchTenantBranding(tenantCode);
    } catch {
      notFound();
    }

    snapshot = toFeatureSnapshot(tenant);
    // A tenant with no branding configured yet returns empty strings. Emitting
    // `--color-brand: ;` is a *valid* custom property holding an empty value, so
    // every var(--color-brand) reference resolves to nothing and silently blanks
    // the primary button, focus rings and accents. Fall back to the platform brand.
    const configuredBrand = tenant.branding.brand.primary?.trim();
    const configuredSecondary = tenant.branding.brand.secondary?.trim();
    brand = configuredBrand ? configuredBrand : DEFAULT_BRAND;
    secondary = configuredSecondary;
  }

  const tenantTheme = `:root { --color-brand: ${brand}; --color-brand-contrast: ${brandContrastColor(brand)}; --color-brand-on-dark: ${brandOnDarkColor(brand)};${secondary ? ` --color-brand-secondary: ${secondary};` : ""} }`;

  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: bootScript }} />
        <style id="tenant-theme" dangerouslySetInnerHTML={{ __html: tenantTheme }} />
      </head>
      <body className="bg-background text-foreground">
        <FeatureFlagsProvider snapshot={snapshot}>{children}</FeatureFlagsProvider>
      </body>
    </html>
  );
}

function isGlobalAuthEntry(pathname: string): boolean {
  return ["/login", "/accept-invite", "/forgot-password", "/reset-password"].includes(pathname);
}
